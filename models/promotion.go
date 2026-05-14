// LOKASI: kasirpintar-api/models/promotion.go
package models

import (
	"time"

	"gorm.io/gorm"
)

type PromotionType string

const (
	PERCENTAGE   PromotionType = "PERCENTAGE"
	FIXED_AMOUNT PromotionType = "FIXED_AMOUNT"
	BOGO         PromotionType = "BOGO"
)

type PromotionStatus string

const (
	ACTIVE PromotionStatus = "ACTIVE"
	// --- PERBAIKAN UTAMA DI SINI ---
	INACTIVE PromotionStatus = "INACTIVE" // Tanda kutip penutup ditambahkan
	EXPIRED  PromotionStatus = "EXPIRED"
)

// Promotion mendefinisikan struktur tabel 'promotions'
type Promotion struct {
	ID          uint            `gorm:"primaryKey" json:"ID"`
	CreatedAt   time.Time       `json:"CreatedAt"`
	UpdatedAt   time.Time       `json:"UpdatedAt"`
	DeletedAt   gorm.DeletedAt  `gorm:"index" json:"DeletedAt"`
	Name        string          `gorm:"size:255;not null" json:"Name"`
	Description string          `gorm:"type:text" json:"Description"`
	Type        PromotionType   `gorm:"type:varchar(20);not null" json:"Type"`
	Value       float64         `gorm:"type:decimal(10,2);not null" json:"Value"`
	StartDate   time.Time       `gorm:"not null" json:"start_date"`
	EndDate     time.Time       `gorm:"not null" json:"end_date"`
	Status      PromotionStatus `gorm:"type:varchar(20);not null;default:'INACTIVE'" json:"Status"`
	OutletID    uint            `gorm:"not null" json:"outlet_id"`
	Outlet      Outlet          `gorm:"foreignKey:OutletID" json:"Outlet"`

	// Menghapus tag GORM yang rumit dan membiarkan GORM menggunakan konvensi
	Vouchers []Voucher `json:"Vouchers"`
}

// Voucher mendefinisikan struktur tabel 'vouchers'
type Voucher struct {
	ID                  uint       `gorm:"primaryKey" json:"ID"`
	CreatedAt           time.Time  `json:"CreatedAt"`
	PromotionID         uint       `gorm:"not null" json:"PromotionID"`
	Code                string     `gorm:"size:50;not null;unique" json:"Code"`
	IsUsed              bool       `gorm:"not null;default:false" json:"IsUsed"`
	UsedAt              *time.Time `json:"UsedAt"`
	UsedByTransactionID *uint      `json:"UsedByTransactionID"`
}

// PromotionCustomerSegment mendefinisikan tabel penghubung untuk target segmen
type PromotionCustomerSegment struct {
	PromotionID uint      `gorm:"primaryKey"`
	Promotion   Promotion `gorm:"foreignKey:PromotionID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	SegmentName string `gorm:"primaryKey;size:100"`
}
