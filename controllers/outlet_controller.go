package controllers

import (
	"kasirpintar-api/config"
	"kasirpintar-api/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Struct Input disesuaikan untuk menerima ManagerID (PascalCase)
type OutletInput struct {
	Name      string `json:"Name" binding:"required"`
	Address   string `json:"Address" binding:"required"`
	Phone     string `json:"Phone" binding:"required"`
	ManagerID *uint  `json:"ManagerID"` // Pointer agar bisa menerima null (opsional)
}

func CreateOutlet(c *gin.Context) {
	var input OutletInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Buat outlet baru dengan menyertakan ManagerID
	outlet := models.Outlet{
		Name:      input.Name,
		Address:   input.Address,
		Phone:     input.Phone,
		ManagerID: input.ManagerID,
	}

	if err := config.DB.Create(&outlet).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan outlet."})
		return
	}

	// Ambil kembali data outlet beserta data manajernya untuk respons
	config.DB.Preload("Manager").First(&outlet, outlet.ID)
	c.JSON(http.StatusCreated, gin.H{"data": outlet, "message": "Outlet berhasil dibuat"})
}

func GetAllOutlets(c *gin.Context) {
	var outlets []models.Outlet
	// Gunakan Preload("Manager") untuk mengambil data manajer yang berelasi
	if err := config.DB.Preload("Manager").Find(&outlets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data outlet."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": outlets})
}

func UpdateOutlet(c *gin.Context) {
	outletID := c.Param("id")
	var outlet models.Outlet
	if err := config.DB.First(&outlet, outletID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Outlet tidak ditemukan."})
		return
	}

	var input OutletInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Gunakan map untuk update, ini cara terbaik untuk menangani field opsional (null)
	updateData := map[string]interface{}{
		"Name":      input.Name,
		"Address":   input.Address,
		"Phone":     input.Phone,
		"ManagerID": input.ManagerID,
	}

	if err := config.DB.Model(&outlet).Updates(updateData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui outlet."})
		return
	}

	// Ambil kembali data terbaru beserta manajernya
	config.DB.Preload("Manager").First(&outlet, outletID)
	c.JSON(http.StatusOK, gin.H{"data": outlet, "message": "Outlet berhasil diperbarui"})
}

func DeleteOutlet(c *gin.Context) {
	outletID := c.Param("id")
	var outlet models.Outlet
	if err := config.DB.First(&outlet, outletID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Outlet tidak ditemukan."})
		return
	}

	if err := config.DB.Delete(&outlet).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus outlet."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Outlet berhasil dihapus."})
}

// --- FUNGSI BARU ---
// Endpoint ini akan digunakan oleh frontend untuk mengisi dropdown pilihan manajer
func GetAvailableManagers(c *gin.Context) {
	var managers []models.User
	// Ambil semua user yang memiliki peran 'branch_manager'
	if err := config.DB.Where("role = ?", "branch_manager").Find(&managers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data manajer."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": managers})
}
