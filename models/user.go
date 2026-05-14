package models

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Name     string
	Email    string `gorm:"size:255;not null;unique"`
	Password string `gorm:"size:255;not null" json:"-"`
	Role     string `gorm:"type:enum('admin', 'owner', 'branch_manager', 'cashier');default:'cashier'"`

	// --- PERUBAHAN DI SINI ---
	// 1. Ubah menjadi pointer *uint agar bisa bernilai NULL
	// 2. Hapus gorm:"not null" (jika ada sebelumnya)
	OutletID *uint
	Outlet   *Outlet // Pointer ini sudah benar untuk memutus siklus
}
