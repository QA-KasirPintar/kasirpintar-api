package controllers

import (
	"fmt"
	"kasirpintar-api/config"
	"kasirpintar-api/models"
	"math/rand"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Update Struct Input: Tambahkan OutletIDs
type CreatePromotionInput struct {
	Name        string               `json:"name" binding:"required"`
	Description string               `json:"description"`
	Type        models.PromotionType `json:"type" binding:"required"`
	Value       float64              `json:"value" binding:"required,gt=0"`
	StartDate   time.Time            `json:"start_date" binding:"required"`
	EndDate     time.Time            `json:"end_date" binding:"required,gtfield=StartDate"`
	VoucherQty  int                  `json:"voucher_qty" binding:"required,gt=0"`
	OutletIDs   []uint               `json:"outlet_ids"` // <-- Tambahan untuk Owner
}

type ApplyVoucherInput struct {
	Code string `json:"code" binding:"required"`
}

type UpdateStatusInput struct {
	Status models.PromotionStatus `json:"status" binding:"required,oneof=ACTIVE INACTIVE"`
}

const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func randomString(n int) string {
	sb := make([]byte, n)
	for i := range sb {
		sb[i] = charset[rand.Intn(len(charset))]
	}
	return string(sb)
}

// --- CreatePromotion: Multi-Outlet Support ---
func CreatePromotion(c *gin.Context) {
	var input CreatePromotionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, _ := c.Get("user")
	currentUser := user.(models.User)

	// 1. Tentukan Target Outlet
	var targetOutletIDs []uint

	if currentUser.Role == "owner" || currentUser.Role == "admin" {
		// Jika Owner/Admin, WAJIB memilih outlet dari input
		if len(input.OutletIDs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Pilih setidaknya satu outlet target."})
			return
		}
		targetOutletIDs = input.OutletIDs
	} else {
		// Jika Staf, otomatis masuk ke outlet mereka sendiri
		if currentUser.OutletID == nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak. Staf tidak punya outlet."})
			return
		}
		targetOutletIDs = []uint{*currentUser.OutletID}
	}

	// 2. Proses Pembuatan Promo & Voucher per Outlet (Atomic Transaction)
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		for _, oid := range targetOutletIDs {
			
			// Cek duplikasi nama promosi PER outlet (Opsional, agar rapi)
			var count int64
			tx.Model(&models.Promotion{}).Where("name = ? AND outlet_id = ?", input.Name, oid).Count(&count)
			if count > 0 {
				// Kita return error agar owner sadar ada nama yang sama
				return fmt.Errorf("promosi '%s' sudah ada di outlet ID %d", input.Name, oid)
			}

			// Buat Object Promotion
			promotion := models.Promotion{
				Name:        input.Name,
				Description: input.Description,
				Type:        input.Type,
				Value:       input.Value,
				StartDate:   input.StartDate,
				EndDate:     input.EndDate,
				Status:      models.INACTIVE, // Default inactive
				OutletID:    oid,
			}

			if err := tx.Create(&promotion).Error; err != nil {
				return err
			}

			// Generate Voucher untuk promosi ini
			var vouchers []models.Voucher
			for i := 0; i < input.VoucherQty; i++ {
				vouchers = append(vouchers, models.Voucher{
					PromotionID: promotion.ID,
					// Buat kode unik. Kita tambahkan prefix random agar aman
					Code:        "PROMO-" + randomString(8), 
					IsUsed:      false,
				})
			}
			if err := tx.Create(&vouchers).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Promosi dan voucher berhasil dibuat di outlet yang dipilih"})
}

// --- GetAllPromotions: Filter berdasarkan Role ---
func GetAllPromotions(c *gin.Context) {
	user, _ := c.Get("user")
	currentUser := user.(models.User)

	var promotions []models.Promotion
	var err error
	
	// Preload Outlet agar Owner bisa lihat promosi ini milik outlet mana
	db := config.DB.Preload("Outlet") 

	if currentUser.Role == "admin" || currentUser.Role == "owner" {
		// Admin & Owner: Ambil SEMUA promosi
		err = db.Find(&promotions).Error
	} else if currentUser.Role == "branch_manager" || currentUser.Role == "cashier" {
		// Staf Outlet: Hanya ambil promosi dari outlet mereka
		if currentUser.OutletID == nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "Staf tidak terikat ke outlet."})
			return
		}
		err = db.Where("outlet_id = ?", *currentUser.OutletID).Find(&promotions).Error
	} else {
		c.JSON(http.StatusForbidden, gin.H{"error": "Peran pengguna tidak valid."})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data promosi."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": promotions})
}

// ApplyVoucher: Validasi Outlet Diperketat
func ApplyVoucher(c *gin.Context) {
	var input ApplyVoucherInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var voucher models.Voucher
	if err := config.DB.Where("code = ?", input.Code).First(&voucher).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Kode voucher tidak ditemukan."})
		return
	}
	if voucher.IsUsed {
		c.JSON(http.StatusConflict, gin.H{"error": "Voucher ini sudah pernah digunakan."})
		return
	}
	var promotion models.Promotion
	if err := config.DB.First(&promotion, voucher.PromotionID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil detail promosi terkait voucher."})
		return
	}

	// Validasi outlet (PENTING agar voucher Jaksel tidak dipakai di Jakpus)
	user, _ := c.Get("user")
	currentUser := user.(models.User)
	
	if currentUser.OutletID == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Hanya staf outlet yang bisa menerapkan voucher."})
		return
	}
	if *currentUser.OutletID != promotion.OutletID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Voucher ini tidak berlaku di outlet ini."})
		return
	}

	now := time.Now()
	if promotion.Status != models.ACTIVE {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Promosi untuk voucher ini tidak aktif."})
		return
	}
	if now.Before(promotion.StartDate) || now.After(promotion.EndDate) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Voucher tidak berlaku pada periode ini."})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"message":    "Voucher berhasil diterapkan",
			"voucher_id": voucher.ID,
			"code":       voucher.Code,
			"promotion":  promotion,
		},
	})
}

// --- UpdatePromotionStatus ---
func UpdatePromotionStatus(c *gin.Context) {
	promoID := c.Param("id")
	var input UpdateStatusInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, _ := c.Get("user")
	currentUser := user.(models.User)
	var promotion models.Promotion
	var err error
	
	query := config.DB.Where("id = ?", promoID)

	// Filter akses
	if currentUser.Role != "admin" && currentUser.Role != "owner" {
		if currentUser.OutletID == nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "Staf tidak terikat ke outlet."})
			return
		}
		query = query.Where("outlet_id = ?", *currentUser.OutletID)
	}

	if err = query.First(&promotion).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Promosi tidak ditemukan atau akses ditolak."})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mencari promosi."})
		return
	}

	if err := config.DB.Model(&promotion).Update("status", input.Status).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui status promosi."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": promotion})
}

// --- GetPromotionByID ---
func GetPromotionByID(c *gin.Context) {
	promoID := c.Param("id")
	user, _ := c.Get("user")
	currentUser := user.(models.User)

	var promotion models.Promotion
	var err error
	
	// Selalu preload Vouchers & Outlet
	db := config.DB.Preload("Vouchers").Preload("Outlet") 

	if currentUser.Role == "admin" || currentUser.Role == "owner" {
		err = db.First(&promotion, promoID).Error
	} else if currentUser.Role == "branch_manager" || currentUser.Role == "cashier" {
		if currentUser.OutletID == nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "Staf tidak terikat ke outlet."})
			return
		}
		err = db.Where("id = ? AND outlet_id = ?", promoID, *currentUser.OutletID).First(&promotion).Error
	} else {
		c.JSON(http.StatusForbidden, gin.H{"error": "Peran pengguna tidak valid."})
		return
	}

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Promosi tidak ditemukan."})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil detail promosi."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": promotion})
}