package controllers

import (
	"fmt"
	"kasirpintar-api/config"
	"kasirpintar-api/models"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// --- PERBAIKAN 1: Update Struct untuk menerima array OutletIDs ---
type CreateMenuInput struct {
	Name      string  `json:"name" binding:"required"`
	Price     float64 `json:"price" binding:"required,gt=0"`
	Category  string  `json:"category"`
	Stock     int     `json:"stock" binding:"gte=0"`
	OutletIDs []uint  `json:"outlet_ids"` // <-- Field Baru (Array)
}

type UpdateMenuInput struct {
	Name     string  `json:"name"`
	Price    float64 `json:"price" binding:"omitempty,gt=0"`
	Category string  `json:"category"`
	Stock    int     `json:"stock" binding:"omitempty,gte=0"`
}

func GetAllMenus(c *gin.Context) {
    user, exists := c.Get("user")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "User tidak terautentikasi"})
        return
    }
    currentUser := user.(models.User)

    var menus []models.Menu
    var err error

    // --- PERBAIKAN DI SINI ---
    // Tambahkan .Preload("Outlet") agar GORM mengambil data outlet terkait
    db := config.DB.Preload("Outlet") 

    // Logika Role
    if currentUser.Role == "admin" || currentUser.Role == "owner" {
        // Admin & Owner: Ambil SEMUA menu dari SEMUA outlet
        err = db.Find(&menus).Error
    } else if currentUser.Role == "branch_manager" || currentUser.Role == "cashier" {
        // Staf Outlet: Hanya ambil menu dari outlet mereka
        if currentUser.OutletID == nil {
            c.JSON(http.StatusForbidden, gin.H{"error": "Staf tidak terikat ke outlet."})
            return
        }
        err = db.Where("outlet_id = ?", *currentUser.OutletID).Find(&menus).Error
    } else {
        c.JSON(http.StatusForbidden, gin.H{"error": "Peran pengguna tidak valid."})
        return
    }

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data menu."})
        return
    }

    c.JSON(http.StatusOK, gin.H{"data": menus})
}

// --- PERBAIKAN 2: Logika CreateMenu Multi-Outlet ---
func CreateMenu(c *gin.Context) {
	var input CreateMenuInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, _ := c.Get("user")
	currentUser := user.(models.User)

	// Tentukan target outlet (bisa satu atau banyak)
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

	// Gunakan Transaction untuk memastikan konsistensi data
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		for _, oid := range targetOutletIDs {
			// Cek duplikasi nama PER outlet
			var count int64
			tx.Model(&models.Menu{}).Where("name = ? AND outlet_id = ?", input.Name, oid).Count(&count)
			if count > 0 {
				// Kita return error agar proses berhenti dan memberitahu user
				// Opsional: Anda bisa menggunakan 'continue' jika ingin skip yang duplikat saja
				return fmt.Errorf("menu '%s' sudah ada di outlet ID %d", input.Name, oid)
			}

			menu := models.Menu{
				Name:     input.Name,
				Price:    input.Price,
				Category: input.Category,
				Stock:    input.Stock,
				OutletID: oid,
			}
			if err := tx.Create(&menu).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Menu berhasil dibuat."})
}

// --- PERBAIKAN 3: Logika UpdateMenu (Fix Error 403 Owner) ---
func UpdateMenu(c *gin.Context) {
	menuID := c.Param("id")
	var input UpdateMenuInput
	if err := c.BindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid: " + err.Error()})
		return
	}
	user, _ := c.Get("user")
	currentUser := user.(models.User)

	var menu models.Menu
	
	// Query mencari menu
	query := config.DB.Where("id = ?", menuID)

	// Branch Manager & Cashier hanya bisa edit menu di outlet mereka
	if currentUser.Role != "owner" && currentUser.Role != "admin" {
		if currentUser.OutletID == nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak."})
			return
		}
		query = query.Where("outlet_id = ?", *currentUser.OutletID)
	}

	if err := query.First(&menu).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Menu tidak ditemukan atau akses ditolak."})
		return
	}

	// --- LOGIKA PEMISAHAN HAK AKSES UPDATE ---
	
	// Map untuk menampung data yang AKAN diupdate
	updates := make(map[string]interface{})

	if currentUser.Role == "branch_manager" {
		// ATURAN 1: Branch Manager HANYA BOLEH ubah Stock
		// Abaikan input Name, Price, Category walaupun dikirim
		if input.Stock >= 0 {
			updates["stock"] = input.Stock
		} else {
            // Opsional: Validasi stok tidak boleh negatif
        }
		// Kita tidak memasukkan Name, Price, Category ke map 'updates'

	} else if currentUser.Role == "owner" || currentUser.Role == "admin" {
		// ATURAN 2: Owner HANYA BOLEH ubah Nama, Harga, Kategori
		// Stok tidak boleh diubah lewat Edit (harus lewat transaksi atau opname staf)
		if input.Name != "" {
			updates["name"] = input.Name
		}
		if input.Price > 0 {
			updates["price"] = input.Price
		}
		if input.Category != "" {
			updates["category"] = input.Category
		}
		// Kita tidak memasukkan Stock ke map 'updates'
	}

    // Cek duplikasi nama (Khusus Owner saat ganti nama)
	if currentUser.Role == "owner" || currentUser.Role == "admin" {
        if val, ok := updates["name"]; ok && val != menu.Name {
             var count int64
             config.DB.Model(&models.Menu{}).
                Where("name = ? AND outlet_id = ? AND id != ?", val, menu.OutletID, menuID).
                Count(&count)
            if count > 0 {
                c.JSON(http.StatusConflict, gin.H{"error": "Nama menu sudah digunakan di outlet ini."})
                return
            }
        }
    }

	// Lakukan Update hanya pada field yang diizinkan
    // Jika tidak ada update yang valid (map kosong)
    if len(updates) == 0 {
         c.JSON(http.StatusOK, gin.H{"message": "Tidak ada perubahan data yang disimpan.", "data": menu})
         return
    }

	if err := config.DB.Model(&menu).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui menu."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": menu})
}

func DeleteMenu(c *gin.Context) {
	menuID := c.Param("id")
	user, _ := c.Get("user")
	currentUser := user.(models.User)

	var menu models.Menu
	query := config.DB.Where("id = ?", menuID)

	// HANYA FILTER OUTLET JIKA USER ADALAH STAF
	if currentUser.Role != "owner" && currentUser.Role != "admin" {
		if currentUser.OutletID == nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "Akses ditolak."})
			return
		}
		query = query.Where("outlet_id = ?", *currentUser.OutletID)
	}

	if err := query.First(&menu).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Menu tidak ditemukan."})
		return
	}

	if err := config.DB.Delete(&menu).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus menu."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Menu berhasil dihapus."})
}