// LOKASI: models/menu.go
package models

import "gorm.io/gorm"

type Menu struct {
	gorm.Model
	Name        string  `gorm:"size:255;not null"`
	Description string  `gorm:"type:text"`
	Price       float64 `gorm:"not null"`
	Stock       int     `gorm:"default:0"`
	Category    string  `gorm:"size:100;default:'Lainnya'"` // <-- TAMBAHKAN INI
	OutletID    uint    `gorm:"not null"`
	Outlet      Outlet  `gorm:"foreignKey:OutletID"`
}
