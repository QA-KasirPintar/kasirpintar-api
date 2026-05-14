package models

import "gorm.io/gorm"

type Transaction struct {
	gorm.Model
	InvoiceNumber string  `gorm:"size:255;not null;unique"`
	Subtotal      float64 `gorm:"default:0"`
	Discount      float64 `gorm:"default:0"`
	TotalAmount   float64 `gorm:"not null"`

	Status string `gorm:"type:varchar(20);default:'PENDING'"`

	PaymentMethod string  `gorm:"size:50;default:'Tunai'"`
	CashTendered  float64 `gorm:"default:0"`
	Change        float64 `gorm:"default:0"`

	UserID     uint      `json:"user_id"`
	User       User      `gorm:"foreignKey:UserID"`
	CustomerID *uint     `json:"customer_id,omitempty"`
	Customer   *Customer `gorm:"foreignKey:CustomerID"`

	OutletID uint   `json:"outlet_id"`
	Outlet   Outlet `gorm:"foreignKey:OutletID"`

	// Snapshot pajak (baru)
	TaxRateID  *uint    `gorm:"index" json:"tax_rate_id,omitempty"`
	TaxPercent *float64 `gorm:"type:decimal(5,2)" json:"tax_percent,omitempty"`
	TaxAmount  *float64 `gorm:"type:decimal(12,2)" json:"tax_amount,omitempty"`

	Details []TransactionDetail `gorm:"foreignKey:TransactionID"`
}
