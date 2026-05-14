package controllers

import (
	"kasirpintar-api/config"
	"kasirpintar-api/models"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// CreateFirstAdmin hanya bisa dijalankan jika belum ada user admin di database
func CreateFirstAdmin(c *gin.Context) {
	var userCount int64
	config.DB.Model(&models.User{}).Where("role = ?", "admin").Count(&userCount)
	if userCount > 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "Setup sudah selesai, admin sudah ada"})
		return
	}

	var body struct {
		AdminName     string `json:"admin_name" binding:"required"`
		AdminEmail    string `json:"admin_email" binding:"required,email"`
		AdminPassword string `json:"admin_password" binding:"required,min=8"`
		OutletName    string `json:"outlet_name" binding:"required"` // Tetap buat outlet pertama
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "input tidak valid"})
		return
	}

	// Buat outlet pertama (meskipun admin tidak terikat langsung)
	outlet := models.Outlet{Name: body.OutletName}
	if result := config.DB.Create(&outlet); result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membuat outlet pertama"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.AdminPassword), 10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal mengenkripsi password"})
		return
	}

	// --- PERBAIKAN DI SINI ---
	// Admin tidak punya OutletID, jadi set ke nil
	admin := models.User{
		Name:     body.AdminName,
		Email:    body.AdminEmail,
		Password: string(hash),
		Role:     "admin",
		OutletID: nil, // Admin tidak terikat ke outlet
	}
	// --- AKHIR PERBAIKAN ---

	if result := config.DB.Create(&admin); result.Error != nil {
		// Jika gagal buat admin, hapus outlet yang baru dibuat (opsional rollback)
		config.DB.Delete(&outlet)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "gagal membuat user admin"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Setup berhasil! Admin dan outlet pertama telah dibuat."})
}
