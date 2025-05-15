package entity

import (
	"time"
)

type TryLog struct {
	Attempt   int        `json:"attempt"`
	Logs      []LogEntry `json:"logs"`
	Timestamp time.Time  `json:"timestamp"`
}

type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
	Status    string    `json:"status"`
}

type TransactionLog struct {
	TransactionID   string    `json:"transaction_id"`
	Timestamp       time.Time `json:"timestamp"`
	Message         string    `json:"message"`
	TryCount        int       `json:"retry_count,omitempty"`
	Status          string    `json:"status"`
	TransactionLogs []TryLog  `json:"transaction_logs,omitempty"`
}
