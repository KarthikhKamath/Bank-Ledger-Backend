package entity

import (
	"time"

	"gorm.io/gorm"
)

type Transaction struct {
	ID                 string  `gorm:"primaryKey;size:21"`
	AccountID          string  `gorm:"size:21;not null"`
	Amount             float64 `gorm:"type:decimal(20,2);not null"`
	Currency           string  `gorm:"size:3;not null"`
	Type               string  `gorm:"size:20;not null"`
	Status             string  `gorm:"size:20;"`
	Description        string  `gorm:"type:text"`
	ProcessDescription string  `gorm:"type:text"`
	RetryCount         int     `gorm:"default:0"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (t *Transaction) BeforeUpdate(tx *gorm.DB) (err error) {
	t.UpdatedAt = time.Now()
	return
}
