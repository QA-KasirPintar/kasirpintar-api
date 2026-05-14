package models

import "gorm.io/gorm"

type TransactionDetail struct {
	gorm.Model
	TransactionID uint
	MenuID        uint
	Menu          Menu `gorm:"foreignKey:MenuID"` // relasi ke Menu
	Quantity      int
	Price         float64
}
