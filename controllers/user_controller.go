package controllers

import (
	"fmt"
	"kasirpintar-api/config"
	"kasirpintar-api/models"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// --- PERBAIKAN 1: UserInput diubah untuk menerima pointer ---
type UserInput struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password"`
	Role     string `json:"role" binding:"required"`
	OutletID *uint  `json:"outlet_id"` // Diubah ke *uint
}

type ChangePasswordInput struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=6"`
}

func GetAllUsers(c *gin.Context) {
	userToken, _ := c.Get("user")
	currentUser := userToken.(models.User)

	var users []models.User
	db := config.DB.Preload("Outlet").
		Where("role NOT IN (?, ?)", "admin", "owner").
		Where("id != ?", currentUser.ID)

	// --- PERBAIKAN 2: Cek jika currentUser.OutletID tidak nil ---
	if currentUser.Role == "branch_manager" && currentUser.OutletID != nil {
		db = db.Where("outlet_id = ?", *currentUser.OutletID) // Gunakan * untuk mendapatkan nilainya
	}

	if err := db.Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data user."})
		return
	}

	var owner models.User
	// Ambil owner, abaikan error jika tidak ada (FirstOrCreate akan handle ini)
	config.DB.Where("role = ?", "owner").First(&owner) // Owner mungkin belum ada

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"users": users,
			// Kirim owner apa adanya (bisa struct kosong jika belum ada)
			"owner": owner,
		},
	})
}

func GetArchivedUsers(c *gin.Context) {
	userToken, _ := c.Get("user")
	currentUser := userToken.(models.User)

	var users []models.User
	db := config.DB.Unscoped().
		Where("deleted_at IS NOT NULL").
		Preload("Outlet").
		Where("role NOT IN (?, ?)", "admin", "owner").
		Where("id != ?", currentUser.ID)

	// --- PERBAIKAN 3: Cek jika currentUser.OutletID tidak nil ---
	if currentUser.Role == "branch_manager" && currentUser.OutletID != nil {
		db = db.Where("outlet_id = ?", *currentUser.OutletID) // Gunakan * untuk mendapatkan nilainya
	}

	if err := db.Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data arsip user."})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"users": users,
			"owner": nil, // Owner tidak relevan di arsip
		},
	})
}

// --- PERBAIKAN 4: Logika RegisterUser diubah total ---
func RegisterUser(c *gin.Context) {
	var input UserInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid: " + err.Error()})
		return
	}
	userToken, _ := c.Get("user")
	currentUser := userToken.(models.User)

	var outletIDToSet *uint // Gunakan tipe data pointer

	// --- LOGIKA BARU UNTUK MENENTUKAN OutletID ---
	if input.Role == "admin" || input.Role == "owner" {
		// 1. Jika membuat Admin atau Owner, OutletID HARUS NULL.
		outletIDToSet = nil

	} else if input.Role == "branch_manager" || input.Role == "cashier" {
		// 2. Jika membuat Staf (Manajer/Kasir), OutletID WAJIB ADA.
		if currentUser.Role == "owner" || currentUser.Role == "admin" {
			// Owner/Admin harus memilih outlet dari form
			if input.OutletID == nil || *input.OutletID == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Owner/Admin harus memilih outlet untuk staf baru."})
				return
			}
			outletIDToSet = input.OutletID
		} else if currentUser.Role == "branch_manager" {
			// Branch Manager otomatis mendaftarkan staf di outletnya sendiri
			outletIDToSet = currentUser.OutletID
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Peran tidak valid."})
		return
	}
	// --- AKHIR LOGIKA BARU ---

	if input.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password tidak boleh kosong untuk user baru."})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), 10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengenkripsi password"})
		return
	}
	user := models.User{
		Name:     input.Name,
		Email:    input.Email,
		Password: string(hash),
		Role:     input.Role,
		OutletID: outletIDToSet, // Set dengan nilai pointer (bisa nil)
	}
	if err := config.DB.Create(&user).Error; err != nil {
		// Cek error spesifik (misal: email duplikat)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat user. Email mungkin sudah terdaftar."})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": user, "message": "User berhasil dibuat"})
}

// --- PERBAIKAN 5: Logika UpdateUser diubah total ---
func UpdateUser(c *gin.Context) {
	userID := c.Param("id")
	var targetUser models.User
	if err := config.DB.First(&targetUser, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User target tidak ditemukan."})
		return
	}
	var input UserInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid: " + err.Error()})
		return
	}
	userToken, _ := c.Get("user")
	currentUser := userToken.(models.User)

	var outletIDToSet *uint // Gunakan tipe data pointer

	// --- LOGIKA BARU DITERAPKAN DI SINI JUGA ---
	if input.Role == "admin" || input.Role == "owner" {
		// 1. Jika mengubah jadi Admin atau Owner, OutletID HARUS NULL.
		outletIDToSet = nil
	} else if input.Role == "branch_manager" || input.Role == "cashier" {
		// 2. Jika mengubah jadi Staf (Manajer/Kasir), OutletID WAJIB ADA.
		if currentUser.Role == "owner" || currentUser.Role == "admin" {
			// Owner/Admin harus memilih outlet dari form
			if input.OutletID == nil || *input.OutletID == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Owner/Admin harus memilih outlet untuk staf ini."})
				return
			}
			outletIDToSet = input.OutletID
		} else if currentUser.Role == "branch_manager" {
			// Branch Manager hanya bisa mengedit staf di outletnya sendiri
			// Cek pointer dengan aman
			if targetUser.OutletID == nil || currentUser.OutletID == nil || *targetUser.OutletID != *currentUser.OutletID {
				c.JSON(http.StatusForbidden, gin.H{"error": "Manajer hanya dapat mengubah staf di outletnya sendiri."})
				return
			}
			outletIDToSet = currentUser.OutletID
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Peran tidak valid."})
		return
	}
	// --- AKHIR LOGIKA BARU ---

	targetUser.Name = input.Name
	targetUser.Email = input.Email
	targetUser.Role = input.Role
	targetUser.OutletID = outletIDToSet // Set dengan nilai pointer

	if input.Password != "" {
		newHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), 10)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengenkripsi password baru"})
			return
		}
		targetUser.Password = string(newHash)
	}
	if err := config.DB.Save(&targetUser).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui user. Email mungkin sudah digunakan."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": targetUser, "message": "User berhasil diperbarui"})
}

// --- PERBAIKAN 6: Logika Delete, Restore, PermanentDelete diubah ---
func DeleteUser(c *gin.Context) {
	userID := c.Param("id")
	userToken, _ := c.Get("user")
	currentUser := userToken.(models.User)
	if fmt.Sprintf("%d", currentUser.ID) == userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Anda tidak dapat menghapus akun Anda sendiri."})
		return
	}
	var targetUser models.User
	if err := config.DB.First(&targetUser, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User target tidak ditemukan."})
		return
	}
	if currentUser.Role == "branch_manager" && currentUser.OutletID != nil {
		// Cek pointer dengan aman
		if targetUser.OutletID == nil || *targetUser.OutletID != *currentUser.OutletID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Manajer hanya dapat menghapus staf di outletnya sendiri."})
			return
		}
	}
	if err := config.DB.Delete(&targetUser).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus user."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "User berhasil dihapus (diarsipkan)."})
}

func RestoreUser(c *gin.Context) {
	userID := c.Param("id")
	userToken, _ := c.Get("user")
	currentUser := userToken.(models.User)
	var targetUser models.User
	if err := config.DB.Unscoped().First(&targetUser, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User di arsip tidak ditemukan."})
		return
	}
	if currentUser.Role == "branch_manager" && currentUser.OutletID != nil {
		// Cek pointer dengan aman
		if targetUser.OutletID == nil || *targetUser.OutletID != *currentUser.OutletID {
			c.JSON(http.StatusForbidden, gin.H{"error": "Manajer hanya dapat mengembalikan staf di outletnya sendiri."})
			return
		}
	}
	if err := config.DB.Unscoped().Model(&targetUser).Update("deleted_at", nil).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengembalikan user."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "User berhasil diaktifkan kembali."})
}

func PermanentDeleteUser(c *gin.Context) {
	userID := c.Param("id")
	userToken, _ := c.Get("user")
	currentUser := userToken.(models.User)
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		var targetUser models.User
		if err := tx.Unscoped().First(&targetUser, userID).Error; err != nil {
			return err
		}
		if currentUser.Role == "branch_manager" && currentUser.OutletID != nil {
			// Cek pointer dengan aman
			if targetUser.OutletID == nil || *targetUser.OutletID != *currentUser.OutletID {
				return fmt.Errorf("manajer hanya dapat menghapus permanen staf di outletnya sendiri")
			}
		}
		// Hapus relasi transaksi (jika ada)
		// Sesuaikan nama tabel jika berbeda
		// if err := tx.Exec("DELETE FROM transaction_details WHERE transaction_id IN (SELECT id FROM transactions WHERE user_id = ?)", userID).Error; err != nil {
		// 	 return err
		// }
		// if err := tx.Unscoped().Where("user_id = ?", userID).Delete(&models.Transaction{}).Error; err != nil {
		// 	 return err
		// }
		if err := tx.Unscoped().Delete(&targetUser).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if err.Error() == "manajer hanya dapat menghapus permanen staf di outletnya sendiri" {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menghapus user secara permanen: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "User telah dihapus secara permanen."})
}

// --- FUNGSI LAIN TIDAK BERUBAH ---
func ChangePassword(c *gin.Context) {
	var input ChangePasswordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid: " + err.Error()})
		return
	}
	userInterface, _ := c.Get("user")
	currentUser := userInterface.(models.User) // Ambil user dari context

	// Pastikan currentUser didapatkan
	if currentUser.ID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User tidak ditemukan"})
		return
	}

	err := bcrypt.CompareHashAndPassword([]byte(currentUser.Password), []byte(input.CurrentPassword))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password saat ini salah"})
		return
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), 10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengenkripsi password baru"})
		return
	}
	// Update password untuk user yang sedang login
	if err := config.DB.Model(&currentUser).Update("password", string(newHash)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memperbarui password di database"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Password berhasil diubah"})
}
