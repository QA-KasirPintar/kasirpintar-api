package controllers

import (
	"kasirpintar-api/config"
	"kasirpintar-api/models"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Fungsi untuk mencari pelanggan berdasarkan nama atau nomor HP
func GetCustomers(c *gin.Context) {
	user, _ := c.Get("user")
	currentUser := user.(models.User)

	// --- PERBAIKAN 1: Pastikan user punya OutletID ---
	// Endpoint ini hanya untuk user yang terikat ke outlet
	if currentUser.OutletID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak. Pengguna tidak terikat ke outlet tertentu."})
		return
	}

	query := c.Query("search") // Mengambil parameter ?search=... dari URL
	var customers []models.Customer

	// --- PERBAIKAN 2: Gunakan nilai pointer (*currentUser.OutletID) ---
	db := config.DB.Where("outlet_id = ?", *currentUser.OutletID)

	if query != "" {
		searchQuery := "%" + query + "%"
		// MySQL: LIKE sudah case-insensitive secara default
		db = db.Where("name LIKE ? OR phone_number LIKE ?", searchQuery, searchQuery)
	}

	if err := db.Find(&customers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data pelanggan."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": customers}) // Bungkus dalam "data"
}

// Fungsi untuk membuat pelanggan baru
func CreateCustomer(c *gin.Context) {
	user, _ := c.Get("user")
	currentUser := user.(models.User)

	// --- PERBAIKAN 3: Pastikan user punya OutletID ---
	if currentUser.OutletID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak. Hanya staf outlet yang bisa membuat pelanggan."})
		return
	}

	var body struct {
		Name        string `json:"name" binding:"required"`
		PhoneNumber string `json:"phone_number"`
		Email       string `json:"email"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid: Nama wajib diisi"})
		return
	}

	// Trim spasi
	body.Name = strings.TrimSpace(body.Name)
	body.PhoneNumber = strings.TrimSpace(body.PhoneNumber)
	body.Email = strings.TrimSpace(body.Email)

	if body.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Nama pelanggan tidak boleh kosong"})
		return
	}

	// --- PERBAIKAN 4: Gunakan nilai pointer (*currentUser.OutletID) ---
	// Asumsi model Customer.OutletID masih uint biasa
	customer := models.Customer{
		Name:     body.Name,
		OutletID: *currentUser.OutletID, // Ambil nilai uint dari pointer
	}

	if body.PhoneNumber != "" {
		customer.PhoneNumber = body.PhoneNumber
	}
	if body.Email != "" {
		customer.Email = body.Email
	}

	// Cek duplikasi
	if body.PhoneNumber != "" {
		var existing models.Customer
		// --- PERBAIKAN 5: Gunakan nilai pointer di cek duplikat ---
		if config.DB.Where("phone_number = ? AND outlet_id = ?", body.PhoneNumber, *currentUser.OutletID).First(&existing).RowsAffected > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "Nomor HP sudah terdaftar di outlet ini"})
			return
		}
	}
	if body.Email != "" {
		var existing models.Customer
		// --- PERBAIKAN 6: Gunakan nilai pointer di cek duplikat ---
		if config.DB.Where("email = ? AND outlet_id = ?", body.Email, *currentUser.OutletID).First(&existing).RowsAffected > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "Email sudah terdaftar di outlet ini"})
			return
		}
	}

	if result := config.DB.Create(&customer); result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat pelanggan, " + result.Error.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": customer}) // Bungkus dalam "data"
}
