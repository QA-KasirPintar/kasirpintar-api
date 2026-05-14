// models/outlet.go
package models

import "gorm.io/gorm"

// Outlet represents a physical store/outlet
type Outlet struct {
	gorm.Model
	Name    string
	Address string `gorm:"type:text"`
	Phone   string `gorm:"size:20"`

	ManagerID *uint
	Manager   *User `gorm:"foreignKey:ManagerID" json:"manager,omitempty"`

	Users []*User `gorm:"foreignKey:OutletID" json:"-"`

	// TAX fields (optional override per-outlet)
	// TaxRateID points to an entry in tax_rates table (nullable)
	TaxRateID *uint    `gorm:"index" json:"tax_rate_id,omitempty"`
	TaxRate   *TaxRate `gorm:"foreignKey:TaxRateID" json:"tax_rate,omitempty"`

	// RegionName used to lookup tax by region if TaxRateID not set
	RegionName *string `gorm:"size:191;index" json:"region_name,omitempty"`
}
