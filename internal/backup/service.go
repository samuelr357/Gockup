package backup

import (
	"archive/zip"
	"context"
	"database/sql"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"mysql-backup/internal/config"
	"mysql-backup/internal/google"
	"mysql-backup/internal/ssh"

	_ "github.com/go-sql-driver/mysql"
)

type Service struct {
	config *config.Config
}

type BackupResult struct {
	Database string `json:"database"`
	Success  bool   `json:"success"`
	Error    string `json:"error,omitempty"`
	FileName string `json:"file_name,omitempty"`
	FileSize int64  `json:"file_size,omitempty"`
}

type MachineBackupResult struct {
	MachineID string         `json:"machine_id"`
	Machine   string         `json:"machine"`
	Results   []BackupResult `json:"results"`
	Success   bool           `json:"success"`
	Error     string         `json:"error,omitempty"`
}

func NewService(cfg *config.Config) *Service {
	return &Service{
		config: cfg,
	}
}

// sanitizeName removes special characters and spaces from machine names for file naming
func sanitizeName(name string) string {
	// Replace spaces and special characters with underscores
	reg := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	sanitized := reg.ReplaceAllString(name, "_")

	// Remove multiple consecutive underscores
	reg2 := regexp.MustCompile(`_+`)
	sanitized = reg2.ReplaceAllString(sanitized, "_")

	// Remove leading/trailing underscores
	sanitized = strings.Trim(sanitized, "_")

	// If empty after sanitization, use "server"
	if sanitized == "" {
		sanitized = "server"
	}

	return sanitized
}

func (s *Service) TestMachineConnection(machineID string) error {
	machine, err := s.config.GetMachine(machineID)
	if err != nil {
		return err
	}

	fmt.Printf("Testing connection for machine: %s (%s)\n", machine.Name, machine.Type)

	if machine.Type == "remote" {
		// Test SSH connection first
		fmt.Printf("Testing SSH connection to %s@%s:%d\n", machine.SSH.Username, machine.SSH.Host, machine.SSH.Port)
		sshClient := ssh.NewClient(&machine.SSH)
		if err := sshClient.TestConnection(); err != nil {
			return fmt.Errorf("SSH connection failed: %w", err)
		}
		fmt.Println("SSH connection successful!")

		// Test if MySQL is running on remote server
		fmt.Printf("Testing if MySQL is running on remote server...\n")
		if err := s.testRemoteMySQLService(machine); err != nil {
			return fmt.Errorf("MySQL service check failed: %w", err)
		}
		fmt.Println("MySQL service is running on remote server!")
	}

	// Test MySQL connection
	fmt.Printf("Testing MySQL connection to %s@%s:%d\n", machine.MySQL.Username, machine.MySQL.Host, machine.MySQL.Port)
	return s.testMySQLConnection(machine)
}

func (s *Service) testRemoteMySQLService(machine *config.Machine) error {
	sshClient := ssh.NewClient(&machine.SSH)
	if err := sshClient.Connect(); err != nil {
		return fmt.Errorf("failed to connect SSH: %w", err)
	}
	defer sshClient.Close()

	// Check if MySQL is running
	commands := []string{
		"systemctl is-active mysql",
		"systemctl is-active mysqld",
		"service mysql status",
		"pgrep mysqld",
		fmt.Sprintf("netstat -ln | grep :%d", machine.MySQL.Port),
		fmt.Sprintf("ss -ln | grep :%d", machine.MySQL.Port),
	}

	for _, cmd := range commands {
		fmt.Printf("SSH: Running command: %s\n", cmd)
		output, err := sshClient.ExecuteCommand(cmd)
		if err == nil && len(output) > 0 {
			fmt.Printf("SSH: Command successful: %s\n", strings.TrimSpace(string(output)))
			return nil
		}
		fmt.Printf("SSH: Command failed or no output: %v\n", err)
	}

	return fmt.Errorf("MySQL service does not appear to be running on remote server")
}

func (s *Service) GetMachineDatabases(machineID string) ([]string, error) {
	machine, err := s.config.GetMachine(machineID)
	if err != nil {
		return nil, err
	}

	return s.getDatabasesForMachine(machine)
}

func (s *Service) CreateMachineBackup(ctx context.Context, machineID string, databases []string) ([]BackupResult, error) {
	machine, err := s.config.GetMachine(machineID)
	if err != nil {
		return nil, err
	}

	return s.createBackupForMachine(ctx, machine, databases)
}

func (s *Service) testMySQLConnection(machine *config.Machine) error {
	var dsn string
	var cleanup func()

	if machine.Type == "remote" {
		// Create SSH tunnel for remote connection
		localPort, tunnelCleanup, err := s.createSSHTunnel(machine)
		if err != nil {
			return fmt.Errorf("failed to create SSH tunnel: %w", err)
		}
		cleanup = tunnelCleanup
		defer cleanup()

		// MySQL connection through SSH tunnel uses localhost and tunnel port
		dsn = fmt.Sprintf("%s:%s@tcp(localhost:%s)/",
			machine.MySQL.Username, // MySQL user (ex: root, admin, etc)
			machine.MySQL.Password, // MySQL password
			localPort,              // Local tunnel port
		)
		fmt.Printf("MySQL: Connecting through SSH tunnel on localhost:%s as MySQL user '%s'\n", localPort, machine.MySQL.Username)
	} else {
		// Direct MySQL connection for local machines
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/",
			machine.MySQL.Username, // MySQL user
			machine.MySQL.Password, // MySQL password
			machine.MySQL.Host,     // MySQL host
			machine.MySQL.Port,     // MySQL port
		)
		fmt.Printf("MySQL: Direct connection to %s:%d as MySQL user '%s'\n", machine.MySQL.Host, machine.MySQL.Port, machine.MySQL.Username)
	}

	fmt.Printf("MySQL: Opening connection with DSN: %s:***@tcp(...)\n", machine.MySQL.Username)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to open MySQL connection: %w", err)
	}
	defer db.Close()

	// Set connection timeout
	db.SetConnMaxLifetime(30 * time.Second)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(0)

	fmt.Printf("MySQL: Pinging database...\n")
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping MySQL server: %w", err)
	}

	fmt.Println("MySQL connection successful!")
	return nil
}

func (s *Service) getDatabasesForMachine(machine *config.Machine) ([]string, error) {
	var dsn string
	var cleanup func()

	if machine.Type == "remote" {
		// Create SSH tunnel for remote connection
		localPort, tunnelCleanup, err := s.createSSHTunnel(machine)
		if err != nil {
			return nil, fmt.Errorf("failed to create SSH tunnel: %w", err)
		}
		cleanup = tunnelCleanup
		defer cleanup()

		dsn = fmt.Sprintf("%s:%s@tcp(localhost:%s)/",
			machine.MySQL.Username,
			machine.MySQL.Password,
			localPort,
		)
	} else {
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/",
			machine.MySQL.Username,
			machine.MySQL.Password,
			machine.MySQL.Host,
			machine.MySQL.Port,
		)
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Set connection timeout
	db.SetConnMaxLifetime(30 * time.Second)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(0)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	rows, err := db.Query("SHOW DATABASES")
	if err != nil {
		return nil, fmt.Errorf("failed to get databases: %w", err)
	}
	defer rows.Close()

	var databases []string
	systemDatabases := map[string]bool{
		"information_schema": true,
		"performance_schema": true,
		"mysql":              true,
		"sys":                true,
	}

	for rows.Next() {
		var database string
		if err := rows.Scan(&database); err != nil {
			return nil, fmt.Errorf("failed to scan database name: %w", err)
		}

		// Skip system databases
		if !systemDatabases[database] {
			databases = append(databases, database)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over databases: %w", err)
	}

	return databases, nil
}

func (s *Service) createBackupForMachine(ctx context.Context, machine *config.Machine, databases []string) ([]BackupResult, error) {
	fmt.Printf("Starting backup process for machine %s (%s) for %d databases: %v\n", machine.ID, machine.Name, len(databases), databases)

	// Sanitize machine name for file naming
	sanitizedMachineName := sanitizeName(machine.Name)
	fmt.Printf("Using sanitized machine name for files: %s\n", sanitizedMachineName)

	// Ensure backup directory exists
	backupPath := filepath.Join(s.config.Backup.LocalPath, machine.ID)
	if err := os.MkdirAll(backupPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}
	fmt.Printf("Backup directory: %s\n", backupPath)

	var results []BackupResult
	timestamp := time.Now().Format("20060102_150405")

	// Setup connection parameters
	var mysqlHost string
	var mysqlPort int
	var cleanup func()

	if machine.Type == "remote" {
		// Create SSH tunnel for remote connection
		localPort, tunnelCleanup, err := s.createSSHTunnel(machine)
		if err != nil {
			return nil, fmt.Errorf("failed to create SSH tunnel: %w", err)
		}
		cleanup = tunnelCleanup
		defer cleanup()

		mysqlHost = "localhost"
		port, _ := strconv.Atoi(localPort)
		mysqlPort = port
		fmt.Printf("Using SSH tunnel: localhost:%d -> %s:%d\n", mysqlPort, machine.MySQL.Host, machine.MySQL.Port)
	} else {
		mysqlHost = machine.MySQL.Host
		mysqlPort = machine.MySQL.Port
		fmt.Printf("Direct MySQL connection: %s:%d\n", mysqlHost, mysqlPort)
	}

	for _, database := range databases {
		fmt.Printf("\n=== Processing database: %s on machine %s ===\n", database, machine.Name)
		result := BackupResult{Database: database}

		// Create backup for this database using machine name instead of ID
		fileName := fmt.Sprintf("backup_%s_%s_%s.sql", sanitizedMachineName, database, timestamp)
		filePath := filepath.Join(backupPath, fileName)

		if err := s.dumpDatabaseForMachine(machine, database, filePath, mysqlHost, mysqlPort); err != nil {
			fmt.Printf("ERROR: Failed to dump database %s on machine %s: %v\n", database, machine.Name, err)
			result.Success = false
			result.Error = err.Error()
		} else {
			result.Success = true
			result.FileName = fileName

			// Get file size
			if stat, err := os.Stat(filePath); err == nil {
				result.FileSize = stat.Size()
				fmt.Printf("Backup file created: %s (%.2f MB)\n", fileName, float64(result.FileSize)/(1024*1024))
			}

			// Compress file (sempre comprimir para .sql.gz)
			compressedPath := filePath + ".gz"
			if err := s.compressFileGzip(filePath, compressedPath); err == nil {
				os.Remove(filePath) // Remove uncompressed file
				result.FileName = fileName + ".gz"
				filePath = compressedPath

				// Update file size
				if stat, err := os.Stat(filePath); err == nil {
					result.FileSize = stat.Size()
					fmt.Printf("Compressed file: %s (%.2f MB)\n", result.FileName, float64(result.FileSize)/(1024*1024))
				}
			} else {
				fmt.Printf("WARNING: Failed to compress file %s: %v\n", filePath, err)
				// Continue without compression
			}

			// Upload to Google Drive if configured
			if s.config.IsGoogleAuthenticated() {
				fmt.Printf("Uploading to Google Drive...\n")
				googleClient := google.NewClient(s.config)
				if driveID, err := googleClient.UploadFile(filePath, result.FileName); err == nil {
					fmt.Printf("Successfully uploaded %s to Google Drive (ID: %s)\n", result.FileName, driveID)

					// Log to Google Sheets
					googleClient.LogToSheets(config.BackupLog{
						Timestamp: time.Now(),
						MachineID: machine.ID,
						TableName: database,
						FileName:  result.FileName,
						FileSize:  result.FileSize,
						Success:   true,
						DriveID:   driveID,
					})

					// SEMPRE remover arquivo local após upload bem-sucedido
					os.Remove(filePath)
					fmt.Printf("Local file %s removed after successful upload\n", filePath)
				} else {
					fmt.Printf("WARNING: Failed to upload %s to Google Drive: %v\n", result.FileName, err)
					// Log error to Google Sheets
					googleClient.LogToSheets(config.BackupLog{
						Timestamp: time.Now(),
						MachineID: machine.ID,
						TableName: database,
						FileName:  result.FileName,
						FileSize:  result.FileSize,
						Success:   false,
						Error:     err.Error(),
					})
				}
			} else {
				fmt.Printf("Google Drive not configured, keeping file locally\n")
			}
		}

		// Add log entry
		s.config.AddBackupLog(config.BackupLog{
			Timestamp: time.Now(),
			MachineID: machine.ID,
			TableName: database,
			FileName:  result.FileName,
			FileSize:  result.FileSize,
			Success:   result.Success,
			Error:     result.Error,
		})

		results = append(results, result)
		fmt.Printf("=== Completed database: %s (Success: %v) ===\n", database, result.Success)
	}

	fmt.Printf("\nBackup process completed. Results: %d total\n", len(results))
	return results, nil
}

func (s *Service) createSSHTunnel(machine *config.Machine) (string, func(), error) {
	fmt.Printf("Creating SSH tunnel to %s@%s:%d\n", machine.SSH.Username, machine.SSH.Host, machine.SSH.Port)

	sshClient := ssh.NewClient(&machine.SSH)

	// Find available local port
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return "", nil, fmt.Errorf("failed to find available port: %w", err)
	}
	localPort := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	listener.Close()

	// Create SSH tunnel
	tunnelListener, err := sshClient.CreateTunnel(localPort, machine.MySQL.Host, machine.MySQL.Port)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create SSH tunnel: %w", err)
	}

	cleanup := func() {
		fmt.Println("Closing SSH tunnel...")
		tunnelListener.Close()
		sshClient.Close()
	}

	// Wait longer for tunnel to be ready
	fmt.Printf("Waiting for SSH tunnel to be ready...\n")
	time.Sleep(3 * time.Second)

	fmt.Printf("SSH tunnel established: localhost:%s -> %s:%d\n", localPort, machine.MySQL.Host, machine.MySQL.Port)
	return localPort, cleanup, nil
}

func (s *Service) dumpDatabaseForMachine(machine *config.Machine, database, filePath string, mysqlHost string, mysqlPort int) error {
	fmt.Printf("Creating COMPLETE backup for database: %s on machine: %s\n", database, machine.Name)
	fmt.Printf("Output file: %s\n", filePath)
	fmt.Printf("MySQL connection: %s@%s:%d\n", machine.MySQL.Username, mysqlHost, mysqlPort)

	if _, err := exec.LookPath("mariadb-dump"); err != nil {
		return fmt.Errorf("mariadb-dump not found in PATH: %w", err)
	}

	args := []string{
		"--protocol=TCP",
		"-h", mysqlHost,
		"-P", fmt.Sprintf("%d", mysqlPort),
		"-u", machine.MySQL.Username,
		fmt.Sprintf("-p%s", machine.MySQL.Password),
		"--skip-lock-tables",
		"--routines",
		"--triggers",
		"--no-tablespaces",
		"--default-character-set=utf8mb4",
		"--force",
		"--quick",
		"--max_allowed_packet=64M",
		"--skip-ssl",
		"--databases",
		database,
		"--set-gtid-purged=OFF",
	}

	cmd := exec.Command("mariadb-dump", args...)
	fmt.Println("Executing mariadb-dump with the following parameters:")
	fmt.Println(strings.Join(args, " "))

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	if stderr.Len() > 0 {
		fmt.Printf("mariadb-dump warnings/errors: %s\n", stderr.String())
	}

	if err != nil {
		return fmt.Errorf("mariadb-dump failed: %w", err)
	}

	output := stdout.String()
	fmt.Printf("mariadb-dump output size: %d bytes\n", len(output))

	if len(output) == 0 {
		return fmt.Errorf("mariadb-dump produced empty output")
	}

	filteredOutput := strings.ReplaceAll(output, "DEFINER=", "-- DEFINER=")

	if err := os.WriteFile(filePath, []byte(filteredOutput), 0644); err != nil {
		return fmt.Errorf("failed to write dump file: %w", err)
	}

	fmt.Printf("Backup completed successfully. Dump file saved at: %s\n", filePath)
	return nil
}

// Backward compatibility methods
func (s *Service) TestConnection() error {
	// Get local machine
	localMachine, err := s.config.GetMachine("local")
	if err != nil {
		return err
	}
	return s.testMySQLConnection(localMachine)
}

func (s *Service) GetDatabases() ([]string, error) {
	// Get databases from local machine
	localMachine, err := s.config.GetMachine("local")
	if err != nil {
		return nil, err
	}
	return s.getDatabasesForMachine(localMachine)
}

func (s *Service) GetTables() ([]string, error) {
	return s.GetDatabases()
}

func (s *Service) CreateBackup(ctx context.Context, databases []string) ([]BackupResult, error) {
	// Create backup for local machine
	localMachine, err := s.config.GetMachine("local")
	if err != nil {
		return nil, err
	}
	return s.createBackupForMachine(ctx, localMachine, databases)
}

func (s *Service) compressFileGzip(srcPath, dstPath string) error {
	fmt.Printf("Compressing file: %s -> %s\n", srcPath, dstPath)

	// Verificar se gzip está disponível
	if _, err := exec.LookPath("gzip"); err != nil {
		fmt.Printf("gzip not found, skipping compression\n")
		// Se gzip não estiver disponível, apenas renomeie o arquivo
		return os.Rename(srcPath, strings.TrimSuffix(dstPath, ".gz"))
	}

	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	// Use gzip compression with maximum compression (-9)
	cmd := exec.Command("gzip", "-c")
	cmd.Stdin = srcFile
	cmd.Stdout = dstFile
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gzip compression failed: %w", err)
	}

	fmt.Printf("Compression completed successfully\n")
	return nil
}

func (s *Service) compressFile(srcPath, dstPath string) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	zipWriter := zip.NewWriter(dstFile)
	defer zipWriter.Close()

	writer, err := zipWriter.Create(filepath.Base(srcPath))
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, srcFile)
	return err
}

func (s *Service) CleanupOldBackups() error {
	if s.config.Backup.RetentionDays <= 0 {
		return nil
	}

	cutoff := time.Now().AddDate(0, 0, -s.config.Backup.RetentionDays)

	return filepath.Walk(s.config.Backup.LocalPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && info.ModTime().Before(cutoff) {
			if strings.HasSuffix(path, ".sql") || strings.HasSuffix(path, ".gz") || strings.HasSuffix(path, ".zip") {
				fmt.Printf("Removing old backup: %s\n", path)
				return os.Remove(path)
			}
		}

		return nil
	})
}
