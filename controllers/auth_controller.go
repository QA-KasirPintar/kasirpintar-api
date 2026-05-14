package controllers

import (
	"fmt"
	"kasirpintar-api/config"
	"kasirpintar-api/helpers"
	"kasirpintar-api/models"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5" // Pastikan menggunakan v5 atau sesuai versi Anda
	"golang.org/x/crypto/bcrypt"
)

type LoginInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func Login(c *gin.Context) {
	var input LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid"})
		return
	}

	var user models.User
	// Preload Outlet sudah benar
	if err := config.DB.Preload("Outlet").Where("email = ?", input.Email).First(&user).Error; err != nil {
		// Jangan beri tahu email salah atau password salah secara spesifik
		c.JSON(http.StatusBadRequest, gin.H{"error": "Kredensial tidak valid"})
		return
	}

	// Bandingkan password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Kredensial tidak valid"})
		return
	}

	// --- PERBAIKAN UTAMA: Penanganan OutletID dan Outlet Name untuk Token ---

	// 1. Tentukan nilai outletID untuk token (0 jika nil)
	var outletIDForToken uint = 0 // Default 0 (tidak ada outlet / global)
	if user.OutletID != nil {
		outletIDForToken = *user.OutletID // Ambil nilai uint jika tidak nil
	}

	// 2. Tentukan nama outlet untuk token ("" jika nil)
	var outletNameForToken string = "" // Default string kosong
	if user.Outlet != nil {            // Cek apakah relasi Outlet tidak nil
		outletNameForToken = user.Outlet.Name
	}
	// --- AKHIR PERBAIKAN UTAMA ---

	// Buat token JWT
	claims := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":         user.ID,                                    // Subject (User ID)
		"name":        user.Name,                                  // Nama User
		"email":       user.Email,                                 // Email User
		"role":        user.Role,                                  // Peran User
		"outlet_id":   outletIDForToken,                           // ID Outlet (0 jika global)
		"outlet_name": outletNameForToken,                         // Nama Outlet ("" jika global)
		"exp":         time.Now().Add(time.Hour * 24 * 30).Unix(), // Expire dalam 30 hari
	})

	// Tandatangani token dengan secret key
	secretKey := os.Getenv("JWT_SECRET")
	if secretKey == "" {
		// Log error di server, jangan ekspos ke client
		// log.Fatal("JWT_SECRET environment variable not set")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Konfigurasi server bermasalah"})
		return
	}
	tokenString, err := claims.SignedString([]byte(secretKey))
	if err != nil {
		// Log error di server
		// log.Printf("Error signing token: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat sesi login"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": tokenString})
}

type ForgotPasswordInput struct {
	Email string `json:"email" binding:"required,email"`
}

// ForgotPassword menerima email, membuat token reset, menyimpan hash token di DB, dan mengirim email
func ForgotPassword(c *gin.Context) {
	var input ForgotPasswordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid"})
		return
	}
	// Delegate to helper which handles token generation, storage and sending
	if err := helpers.SendResetEmail(input.Email); err != nil {
		// Log error but do not reveal to client
		fmt.Printf("[forgot password helper error] %v\n", err)
	}
	c.JSON(http.StatusOK, gin.H{"message": "Jika alamat email terdaftar, Anda akan menerima instruksi untuk mereset password."})
}

type ResetPasswordInput struct {
	Email       string `json:"email" binding:"required,email"`
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// ResetPassword memverifikasi token dan mengganti password user
func ResetPassword(c *gin.Context) {
	var input ResetPasswordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid: " + err.Error()})
		return
	}
	// Delegate to helper which verifies token and resets password
	if err := helpers.VerifyAndReset(input.Email, input.Token, input.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Password berhasil diubah. Silakan login dengan password baru."})
}
