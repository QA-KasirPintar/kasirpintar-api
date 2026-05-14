// LOKASI: middlewares/auth_middleware.go (LENGKAP DAN SUDAH DIPERBAIKI)
package middlewares

import (
	"fmt"
	"kasirpintar-api/config"
	"kasirpintar-api/models"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func RequireAuth(c *gin.Context) {
	// --- 🛑 PERBAIKAN DIMULAI DI SINI 🛑 ---

	// 1. Coba ambil token dari Header "Authorization"
	tokenString := c.GetHeader("Authorization")

	// 2. Jika di header tidak ada, Coba ambil dari Query Param "token"
	if tokenString == "" {
		tokenString = c.Query("token")
	} else {
		// Jika ada di header, bersihkan "Bearer "
		if !strings.HasPrefix(tokenString, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "format token harus 'Bearer [token]'"})
			return
		}
		tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	}

	// 3. Jika masih kosong (setelah dicek di 2 tempat), baru error
	if tokenString == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization header dibutuhkan"})
		return
	}

	// --- 🛑 PERBAIKAN SELESAI 🛑 ---

	// Kode validasi token Anda selanjutnya sudah benar,
	// kita hanya perlu memastikan "JWT_SECRET" Anda sudah benar
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		// Pastikan nama "JWT_SECRET" ini sama dengan di file .env Anda
		return []byte(os.Getenv("JWT_SECRET")), nil
	})

	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token tidak valid"})
		return
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		if float64(time.Now().Unix()) > claims["exp"].(float64) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token sudah kadaluarsa"})
			return
		}
		var user models.User
		config.DB.First(&user, claims["sub"])
		if user.ID == 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user pemilik token tidak ditemukan"})
			return
		}
		c.Set("user", user)
		c.Next()
	} else {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token tidak valid"})
	}
}

// Fungsi AuthorizeRole tidak perlu diubah, biarkan seperti versi Anda
func AuthorizeRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userInterface, exists := c.Get("user")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user tidak ditemukan di context"})
			return
		}

		user := userInterface.(models.User)
		isAllowed := false
		for _, role := range allowedRoles {
			if user.Role == role {
				isAllowed = true
				break
			}
		}

		if !isAllowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "akses ditolak"})
			return
		}
		c.Next()
	}
}
