// models/taxrate.go
package models

import "time"

type TaxRate struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Code        string    `gorm:"size:50;unique;not null" json:"code"`
	Region      string    `gorm:"size:191;not null" json:"region"`
	RatePercent float64   `gorm:"type:decimal(5,2);not null;default:0.00" json:"rate_percent"`
	IsActive    bool      `gorm:"default:true" json:"is_active"`
	Note        *string   `gorm:"type:text" json:"note,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
