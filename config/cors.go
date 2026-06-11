package config

import (
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func CORSMiddleware() gin.HandlerFunc {
	return cors.New(cors.Config{
		// AllowOriginFunc digunakan untuk mengizinkan preview deployment Vercel
		// yang domain-nya berubah setiap deploy (contoh: kasirpintar-xxx.vercel.app)
		AllowOriginFunc: func(origin string) bool {
			// Izinkan localhost untuk development
			if origin == "http://localhost:5173" {
				return true
			}
			// Izinkan GitHub Pages
			if origin == "https://kasir-pintar.github.io" {
				return true
			}
			if origin == "https://kasirpintar-beta.vercel.app/" {
				return true
			}
			// Izinkan semua *.vercel.app (production & preview deployments)
			if strings.HasSuffix(origin, ".vercel.app") {
				return true
			}
			return false
		},

		AllowMethods: []string{
			"GET",
			"POST",
			"PUT",
			"PATCH",
			"DELETE",
			"OPTIONS",
		},

		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Authorization",
		},

		ExposeHeaders: []string{
			"Content-Length",
		},

		// BENAR karena kamu pakai Authorization header (JWT)
		AllowCredentials: false,

		MaxAge: 12 * time.Hour,
	})
}
