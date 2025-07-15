package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	Machines  []Machine       `json:"machines"`
	Google    GoogleConfig    `json:"google"`
	Scheduler SchedulerConfig `json:"scheduler"`
	Backup    BackupConfig    `json:"backup"`
	Service   ServiceConfig   `json:"service"`
	filePath  string
	logs      []BackupLog
}

type Machine struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Type        string      `json:"type"` // "local" or "remote"
	Description string      `json:"description"`
	MySQL       MySQLConfig `json:"mysql"`
	SSH         SSHConfig   `json:"ssh,omitempty"`
	Enabled     bool        `json:"enabled"`
	CreatedAt   string      `json:"created_at"`
	UpdatedAt   string      `json:"updated_at"`
}

type MySQLConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Database string `json:"database"`
}

type SSHConfig struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password,omitempty"` // Adicionar este campo
	PrivateKey string `json:"private_key"`
	Passphrase string `json:"passphrase,omitempty"`
	KeyPath    string `json:"key_path,omitempty"`
}

type GoogleConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	SheetID      string `json:"sheet_id"`
	DriveFolder  string `json:"drive_folder"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenExpiry  string `json:"token_expiry"`
}

type SchedulerConfig struct {
	Enabled   bool       `json:"enabled"`
	Schedules []Schedule `json:"schedules"`
}

type Schedule struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Enabled     bool     `json:"enabled"`
	MachineID   string   `json:"machine_id"` // ID da máquina
	Databases   []string `json:"databases"`
	DaysOfWeek  []int    `json:"days_of_week"` // 0=Domingo, 1=Segunda, ..., 6=Sábado
	Times       []string `json:"times"`        // Horários no formato "15:04"
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

type ServiceConfig struct {
	Installed bool `json:"installed"`
}

type BackupConfig struct {
	LocalPath     string `json:"local_path"`
	Compression   bool   `json:"compression"`
	KeepLocal     bool   `json:"keep_local"`
	RetentionDays int    `json:"retention_days"`
}

type BackupLog struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	MachineID string    `json:"machine_id"`
	TableName string    `json:"table_name"`
	FileName  string    `json:"file_name"`
	FileSize  int64     `json:"file_size"`
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
	DriveID   string    `json:"drive_id,omitempty"`
}

func NewConfig(configPath string) (*Config, error) {
	cfg := &Config{
		Machines: []Machine{
			{
				ID:          "local",
				Name:        "Local Machine",
				Type:        "local",
				Description: "Local MySQL server",
				MySQL: MySQLConfig{
					Host: "localhost",
					Port: 3306,
				},
				Enabled:   true,
				CreatedAt: time.Now().Format(time.RFC3339),
				UpdatedAt: time.Now().Format(time.RFC3339),
			},
		},
		Scheduler: SchedulerConfig{
			Enabled:   false,
			Schedules: []Schedule{},
		},
		Backup: BackupConfig{
			LocalPath:     getDefaultBackupPath(),
			Compression:   true,
			KeepLocal:     false,
			RetentionDays: 30,
		},
		logs: make([]BackupLog, 0),
	}

	if configPath == "" {
		configPath = getDefaultConfigPath()
	}
	cfg.filePath = configPath

	// Try to load existing config
	if err := cfg.Load(); err != nil {
		// If config doesn't exist, create default one
		if os.IsNotExist(err) {
			if err := cfg.Save(); err != nil {
				return nil, fmt.Errorf("failed to create default config: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to load config: %w", err)
		}
	}

	return cfg, nil
}

func (c *Config) Load() error {
	data, err := os.ReadFile(c.filePath)
	if err != nil {
		return err
	}

	// Decrypt data
	decrypted, err := c.decrypt(data)
	if err != nil {
		// If decryption fails, try to load as plain JSON (for backward compatibility)
		if err := json.Unmarshal(data, c); err != nil {
			return fmt.Errorf("failed to decrypt and parse config: %w", err)
		}
		return nil
	}

	return json.Unmarshal(decrypted, c)
}

func (c *Config) Save() error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(c.filePath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Encrypt data
	encrypted, err := c.encrypt(data)
	if err != nil {
		return fmt.Errorf("failed to encrypt config: %w", err)
	}

	return os.WriteFile(c.filePath, encrypted, 0600)
}

func (c *Config) encrypt(data []byte) ([]byte, error) {
	key := c.getEncryptionKey()

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

func (c *Config) decrypt(data []byte) ([]byte, error) {
	key := c.getEncryptionKey()

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func (c *Config) getEncryptionKey() []byte {
	// Generate key based on machine-specific information
	hostname, _ := os.Hostname()
	keyString := fmt.Sprintf("mysql-backup-%s", hostname)
	hash := sha256.Sum256([]byte(keyString))
	return hash[:]
}

func (c *Config) IsGoogleConfigured() bool {
	return c.Google.ClientID != "" && c.Google.ClientSecret != ""
}

func (c *Config) IsGoogleAuthenticated() bool {
	return c.Google.AccessToken != "" && c.Google.RefreshToken != ""
}

func (c *Config) AddBackupLog(log BackupLog) error {
	log.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	c.logs = append(c.logs, log)

	// Keep only last 1000 logs
	if len(c.logs) > 1000 {
		c.logs = c.logs[len(c.logs)-1000:]
	}

	return c.Save()
}

func (c *Config) GetBackupLogs() ([]BackupLog, error) {
	// Return logs in reverse chronological order
	logs := make([]BackupLog, len(c.logs))
	for i, log := range c.logs {
		logs[len(c.logs)-1-i] = log
	}
	return logs, nil
}

// Machine management methods
func (c *Config) AddMachine(machine Machine) error {
	machine.ID = fmt.Sprintf("machine_%d", time.Now().UnixNano())
	machine.CreatedAt = time.Now().Format(time.RFC3339)
	machine.UpdatedAt = time.Now().Format(time.RFC3339)

	c.Machines = append(c.Machines, machine)
	return c.Save()
}

func (c *Config) UpdateMachine(machineID string, machine Machine) error {
	for i, m := range c.Machines {
		if m.ID == machineID {
			machine.ID = machineID
			machine.CreatedAt = m.CreatedAt
			machine.UpdatedAt = time.Now().Format(time.RFC3339)
			c.Machines[i] = machine
			return c.Save()
		}
	}
	return fmt.Errorf("machine not found")
}

func (c *Config) DeleteMachine(machineID string) error {
	// Don't allow deleting local machine
	if machineID == "local" {
		return fmt.Errorf("cannot delete local machine")
	}

	for i, m := range c.Machines {
		if m.ID == machineID {
			c.Machines = append(c.Machines[:i], c.Machines[i+1:]...)

			// Remove schedules for this machine
			var newSchedules []Schedule
			for _, s := range c.Scheduler.Schedules {
				if s.MachineID != machineID {
					newSchedules = append(newSchedules, s)
				}
			}
			c.Scheduler.Schedules = newSchedules

			return c.Save()
		}
	}
	return fmt.Errorf("machine not found")
}

func (c *Config) GetMachine(machineID string) (*Machine, error) {
	for _, m := range c.Machines {
		if m.ID == machineID {
			return &m, nil
		}
	}
	return nil, fmt.Errorf("machine not found")
}

func (c *Config) GetEnabledMachines() []Machine {
	var enabled []Machine
	for _, m := range c.Machines {
		if m.Enabled {
			enabled = append(enabled, m)
		}
	}
	return enabled
}

// Schedule management methods
func (c *Config) AddSchedule(schedule Schedule) error {
	schedule.ID = fmt.Sprintf("schedule_%d", time.Now().UnixNano())
	schedule.CreatedAt = time.Now().Format(time.RFC3339)
	schedule.UpdatedAt = time.Now().Format(time.RFC3339)

	c.Scheduler.Schedules = append(c.Scheduler.Schedules, schedule)
	return c.Save()
}

func (c *Config) UpdateSchedule(scheduleID string, schedule Schedule) error {
	for i, s := range c.Scheduler.Schedules {
		if s.ID == scheduleID {
			schedule.ID = scheduleID
			schedule.CreatedAt = s.CreatedAt
			schedule.UpdatedAt = time.Now().Format(time.RFC3339)
			c.Scheduler.Schedules[i] = schedule
			return c.Save()
		}
	}
	return fmt.Errorf("schedule not found")
}

func (c *Config) DeleteSchedule(scheduleID string) error {
	for i, s := range c.Scheduler.Schedules {
		if s.ID == scheduleID {
			c.Scheduler.Schedules = append(c.Scheduler.Schedules[:i], c.Scheduler.Schedules[i+1:]...)
			return c.Save()
		}
	}
	return fmt.Errorf("schedule not found")
}

func (c *Config) GetSchedule(scheduleID string) (*Schedule, error) {
	for _, s := range c.Scheduler.Schedules {
		if s.ID == scheduleID {
			return &s, nil
		}
	}
	return nil, fmt.Errorf("schedule not found")
}

func (c *Config) GetEnabledSchedules() []Schedule {
	var enabled []Schedule
	for _, s := range c.Scheduler.Schedules {
		if s.Enabled {
			enabled = append(enabled, s)
		}
	}
	return enabled
}

func (c *Config) GetSchedulesForMachine(machineID string) []Schedule {
	var schedules []Schedule
	for _, s := range c.Scheduler.Schedules {
		if s.MachineID == machineID {
			schedules = append(schedules, s)
		}
	}
	return schedules
}

func getDefaultConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".mysql-backup-config"
	}
	return filepath.Join(homeDir, "config/.mysql-backup-config")
}

func getDefaultBackupPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "./mysql-backups"
	}
	return filepath.Join(homeDir, "mysql-backups")
}
