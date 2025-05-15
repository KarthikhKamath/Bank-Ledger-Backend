package entity

import (
	"time"

	"gorm.io/gorm"
)

type Account struct {
	ID            string  `gorm:"primaryKey;size:21"`
	AccountNumber string  `gorm:"size:100;uniqueIndex;not null"`
	Name          string  `gorm:"size:255;not null"`
	Balance       float64 `gorm:"type:decimal(20,2);not null"`
	Currency      string  `gorm:"size:3;not null"`
	Status        string  `gorm:"size:20;default:ACTIVE"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (a *Account) BeforeCreate(tx *gorm.DB) (err error) {
	now := time.Now()
	a.CreatedAt = now
	a.UpdatedAt = now
	return
}

func (a *Account) BeforeUpdate(tx *gorm.DB) (err error) {
	a.UpdatedAt = time.Now()
	return
}
