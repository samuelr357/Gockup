package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"mysql-backup/internal/backup"
	"mysql-backup/internal/config"

	"github.com/robfig/cron/v3"
)

type Service struct {
	config        *config.Config
	backupService *backup.Service
	cron          *cron.Cron
	running       bool
	mu            sync.RWMutex
	nextRuns      map[string]time.Time // ID do agendamento -> próxima execução
}

func NewService(cfg *config.Config, backupService *backup.Service) *Service {
	return &Service{
		config:        cfg,
		backupService: backupService,
		cron:          cron.New(),
		nextRuns:      make(map[string]time.Time),
	}
}

func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("scheduler is already running")
	}

	// Obter agendamentos habilitados
	schedules := s.config.GetEnabledSchedules()
	if len(schedules) == 0 {
		return fmt.Errorf("no enabled schedules found")
	}

	log.Printf("Starting scheduler with %d enabled schedules", len(schedules))

	// Adicionar cada agendamento ao cron
	for _, schedule := range schedules {
		if err := s.addScheduleToCron(schedule); err != nil {
			log.Printf("Failed to add schedule %s to cron: %v", schedule.Name, err)
			continue
		}
		log.Printf("Added schedule: %s", schedule.Name)
	}

	// Start cron scheduler
	s.cron.Start()
	s.running = true

	log.Println("Scheduler started successfully")
	return nil
}

func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.cron.Stop()
	s.running = false
	s.nextRuns = make(map[string]time.Time)

	log.Println("Scheduler stopped")
}

func (s *Service) Restart() error {
	s.Stop()
	return s.Start(context.Background())
}

func (s *Service) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

func (s *Service) GetNextRuns() map[string]time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]time.Time)
	for k, v := range s.nextRuns {
		result[k] = v
	}
	return result
}

func (s *Service) addScheduleToCron(schedule config.Schedule) error {
	// Para cada dia da semana e horário, criar uma entrada no cron
	for _, dayOfWeek := range schedule.DaysOfWeek {
		for _, timeStr := range schedule.Times {
			cronExpr, err := s.buildCronExpression(dayOfWeek, timeStr)
			if err != nil {
				return fmt.Errorf("failed to build cron expression: %w", err)
			}

			// Criar função de callback que captura o agendamento
			scheduleFunc := func(sched config.Schedule) func() {
				return func() {
					s.runScheduledBackup(sched)
				}
			}(schedule)

			entryID, err := s.cron.AddFunc(cronExpr, scheduleFunc)
			if err != nil {
				return fmt.Errorf("failed to add cron job: %w", err)
			}

			// Calcular próxima execução
			entries := s.cron.Entries()
			for _, entry := range entries {
				if entry.ID == entryID {
					s.nextRuns[fmt.Sprintf("%s_%d_%s", schedule.ID, dayOfWeek, timeStr)] = entry.Next
					break
				}
			}

			log.Printf("Added cron job for schedule '%s': %s (Day: %d, Time: %s)",
				schedule.Name, cronExpr, dayOfWeek, timeStr)
		}
	}

	return nil
}

func (s *Service) buildCronExpression(dayOfWeek int, timeStr string) (string, error) {
	// Parse time string (formato "15:04")
	t, err := time.Parse("15:04", timeStr)
	if err != nil {
		return "", fmt.Errorf("invalid time format: %s", timeStr)
	}

	hour := t.Hour()
	minute := t.Minute()

	// Construir expressão cron: "minuto hora * * dia_da_semana"
	cronExpr := fmt.Sprintf("%d %d * * %d", minute, hour, dayOfWeek)
	return cronExpr, nil
}

func (s *Service) runScheduledBackup(schedule config.Schedule) {
	log.Printf("Starting scheduled backup: %s (%d databases) on machine %s", schedule.Name, len(schedule.Databases), schedule.MachineID)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	results, err := s.backupService.CreateMachineBackup(ctx, schedule.MachineID, schedule.Databases)
	if err != nil {
		log.Printf("Scheduled backup '%s' failed: %v", schedule.Name, err)
		return
	}

	successCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		} else {
			log.Printf("Backup failed for database %s in schedule '%s': %s",
				result.Database, schedule.Name, result.Error)
		}
	}

	log.Printf("Scheduled backup '%s' completed: %d/%d databases successful",
		schedule.Name, successCount, len(results))

	// Clean up old backups
	if err := s.backupService.CleanupOldBackups(); err != nil {
		log.Printf("Failed to cleanup old backups: %v", err)
	}
}

// Métodos de compatibilidade com o sistema antigo
func (s *Service) GetNextRun() *time.Time {
	nextRuns := s.GetNextRuns()
	if len(nextRuns) == 0 {
		return nil
	}

	// Retornar a próxima execução mais próxima
	var earliest *time.Time
	for _, nextRun := range nextRuns {
		if earliest == nil || nextRun.Before(*earliest) {
			earliest = &nextRun
		}
	}

	return earliest
}
