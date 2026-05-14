// LOKASI: kasirpintar-api/models/customer.go
package models

import "gorm.io/gorm"

type Customer struct {
	gorm.Model
	Name        string `gorm:"size:255;not null"`
	PhoneNumber string `gorm:"size:20;unique;default:null"`  // <-- Tambahkan 'default:null'
	Email       string `gorm:"size:255;unique;default:null"` // <-- Tambahkan 'default:null'
	OutletID    uint   `gorm:"not null"`
}
