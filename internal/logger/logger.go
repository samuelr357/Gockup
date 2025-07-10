package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

type Logger struct {
	file *os.File
}

type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
}

func New() *Logger {
	homeDir, _ := os.UserHomeDir()
	logDir := filepath.Join(homeDir, ".mysql-backup-logs")
	os.MkdirAll(logDir, 0755)
	
	logFile := filepath.Join(logDir, "backup.log")
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("Failed to open log file: %v", err)
		return &Logger{}
	}

	return &Logger{file: file}
}

func (l *Logger) Info(format string, args ...interface{}) {
	l.log("INFO", format, args...)
}

func (l *Logger) Error(format string, args ...interface{}) {
	l.log("ERROR", format, args...)
}

func (l *Logger) Warning(format string, args ...interface{}) {
	l.log("WARNING", format, args...)
}

func (l *Logger) log(level, format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	
	logLine := fmt.Sprintf("[%s] %s: %s\n", timestamp, level, message)
	
	// Log to console
	fmt.Print(logLine)
	
	// Log to file if available
	if l.file != nil {
		l.file.WriteString(logLine)
	}
}

func (l *Logger) Close() {
	if l.file != nil {
		l.file.Close()
	}
}
