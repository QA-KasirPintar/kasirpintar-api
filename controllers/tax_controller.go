// controllers/tax_controller.go
package controllers

import (
	"kasirpintar-api/config"
	"kasirpintar-api/models"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// CreateTaxRateInput represents input payload to create tax rate
type CreateTaxRateInput struct {
	Code        string  `json:"code" binding:"required"`
	Region      string  `json:"region" binding:"required"`
	RatePercent float64 `json:"rate_percent" binding:"required,gte=0"`
	Note        *string `json:"note"`
	IsActive    *bool   `json:"is_active"` // optional
}

// UpdateTaxRateInput allows partial update
type UpdateTaxRateInput struct {
	Code        *string  `json:"code"`
	Region      *string  `json:"region"`
	RatePercent *float64 `json:"rate_percent"`
	Note        *string  `json:"note"`
	IsActive    *bool    `json:"is_active"`
}

// GetAllTaxRates (admin/owner)
func GetAllTaxRates(c *gin.Context) {
	var rates []models.TaxRate
	if err := config.DB.Find(&rates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil daftar tarif pajak", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rates})
}

// CreateTaxRate (admin/owner)
func CreateTaxRate(c *gin.Context) {
	userI, _ := c.Get("user")
	currentUser := userI.(models.User)
	// Only allow owner/admin
	if currentUser.Role != "owner" && currentUser.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak"})
		return
	}

	var input CreateTaxRateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid", "details": err.Error()})
		return
	}

	isActive := true
	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	tr := models.TaxRate{
		Code:        input.Code,
		Region:      input.Region,
		RatePercent: input.RatePercent,
		IsActive:    isActive,
		Note:        input.Note,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := config.DB.Create(&tr).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan tarif pajak", "details": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": tr})
}

// GetTaxRateByID
func GetTaxRateByID(c *gin.Context) {
	idStr := c.Param("id")
	id, _ := strconv.ParseUint(idStr, 10, 64)
	var tr models.TaxRate
	if err := config.DB.First(&tr, uint(id)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tarif pajak tidak ditemukan"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data tarif pajak", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": tr})
}

// UpdateTaxRate
func UpdateTaxRate(c *gin.Context) {
	userI, _ := c.Get("user")
	currentUser := userI.(models.User)
	if currentUser.Role != "owner" && currentUser.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak"})
		return
	}

	idStr := c.Param("id")
	id, _ := strconv.ParseUint(idStr, 10, 64)
	var tr models.TaxRate
	if err := config.DB.First(&tr, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Tarif pajak tidak ditemukan"})
		return
	}

	var input UpdateTaxRateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid", "details": err.Error()})
		return
	}

	if input.Code != nil {
		tr.Code = *input.Code
	}
	if input.Region != nil {
		tr.Region = *input.Region
	}
	if input.RatePercent != nil {
		tr.RatePercent = *input.RatePercent
	}
	if input.Note != nil {
		tr.Note = input.Note
	}
	if input.IsActive != nil {
		tr.IsActive = *input.IsActive
	}
	tr.UpdatedAt = time.Now()

	if err := config.DB.Save(&tr).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui tarif pajak", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": tr})
}

// DeleteTaxRate
func DeleteTaxRate(c *gin.Context) {
	userI, _ := c.Get("user")
	currentUser := userI.(models.User)
	if currentUser.Role != "owner" && currentUser.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak"})
		return
	}

	idStr := c.Param("id")
	id, _ := strconv.ParseUint(idStr, 10, 64)
	if err := config.DB.Delete(&models.TaxRate{}, uint(id)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus tarif pajak", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Tarif pajak dihapus"})
}
