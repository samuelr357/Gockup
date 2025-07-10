package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"mysql-backup/internal/api"
	"mysql-backup/internal/backup"
	"mysql-backup/internal/config"
	"mysql-backup/internal/scheduler"
	"mysql-backup/internal/service"
)

const (
	AppName    = "MySQL Backup System"
	AppVersion = "1.0.0"
)

func main() {
	var (
		port        = flag.Int("port", 8030, "HTTP server port")
		configPath  = flag.String("config", "", "Path to config file")
		showVersion = flag.Bool("version", false, "Show version information")
		daemon      = flag.Bool("daemon", false, "Run as daemon (service mode)")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("%s v%s\n", AppName, AppVersion)
		return
	}

	fmt.Printf("%s v%s\n", AppName, AppVersion)
	fmt.Println("Starting application...")

	// Load configuration
	cfg, err := config.NewConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize services
	backupService := backup.NewService(cfg)
	schedulerService := scheduler.NewService(cfg, backupService)
	serviceManager := service.NewManager()

	// Initialize API handlers
	handler := api.NewHandler(cfg, backupService, schedulerService, serviceManager)

	// Setup HTTP routes
	mux := http.NewServeMux()

	// Web interface
	mux.HandleFunc("/", handler.IndexHandler)

	// API routes
	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handler.GetConfigHandler(w, r)
		case http.MethodPost:
			handler.UpdateConfigHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/config/test-mysql", handler.TestMySQLHandler)

	mux.HandleFunc("/api/databases", handler.GetDatabasesHandler)

	mux.HandleFunc("/api/backup/manual", handler.CreateManualBackupHandler)
	mux.HandleFunc("/api/backup/logs", handler.GetBackupLogsHandler)

	// Novas rotas para agendamentos
	mux.HandleFunc("/api/schedules", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handler.GetSchedulesHandler(w, r)
		case http.MethodPost:
			handler.CreateScheduleHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/schedules/", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/schedules/") {
			http.NotFound(w, r)
			return
		}

		switch r.Method {
		case "PUT":
			handler.UpdateScheduleHandler(w, r)
		case http.MethodDelete:
			handler.DeleteScheduleHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/scheduler/status", handler.GetSchedulerStatusHandler)
	mux.HandleFunc("/api/scheduler/start", handler.StartSchedulerHandler)
	mux.HandleFunc("/api/scheduler/stop", handler.StopSchedulerHandler)

	mux.HandleFunc("/api/auth/google/url", handler.GetGoogleAuthURLHandler)
	mux.HandleFunc("/api/auth/google/callback", handler.GoogleCallbackHandler)
	mux.HandleFunc("/api/auth/google/status", handler.GetGoogleAuthStatusHandler)

	// Machine routes
	mux.HandleFunc("/api/machines/test-config", handler.TestMachineConfigHandler)

	mux.HandleFunc("/api/machines", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handler.GetMachinesHandler(w, r)
		case http.MethodPost:
			handler.CreateMachineHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/machines/", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/machines/") {
			http.NotFound(w, r)
			return
		}

		if strings.HasSuffix(r.URL.Path, "/test") {
			handler.TestMachineConnectionHandler(w, r)
			return
		}

		if strings.HasSuffix(r.URL.Path, "/databases") {
			handler.GetMachineDatabasesHandler(w, r)
			return
		}

		if strings.HasSuffix(r.URL.Path, "/backup") {
			handler.CreateMachineBackupHandler(w, r)
			return
		}

		switch r.Method {
		case http.MethodPut:
			handler.UpdateMachineHandler(w, r)
		case http.MethodDelete:
			handler.DeleteMachineHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", *port),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start scheduler if there are enabled schedules
	enabledSchedules := cfg.GetEnabledSchedules()
	if len(enabledSchedules) > 0 {
		go func() {
			if err := schedulerService.Start(context.Background()); err != nil {
				log.Printf("Failed to start scheduler: %v", err)
			}
		}()
	}

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		fmt.Println("\nShutting down gracefully...")

		// Stop scheduler
		schedulerService.Stop()

		// Shutdown HTTP server
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
	}()

	// Start HTTP server
	if *daemon {
		fmt.Printf("Running in daemon mode on port %d\n", *port)
	} else {
		fmt.Printf("Web server starting on port %d\n", *port)
		fmt.Printf("Open your browser and go to: http://localhost:%d\n", *port)
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed to start: %v", err)
	}

	fmt.Println("Application stopped.")
}
