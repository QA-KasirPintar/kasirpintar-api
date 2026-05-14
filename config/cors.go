package config

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func CORSMiddleware() gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins: []string{
			"http://localhost:5173",
			"https://kasir-pintar.github.io",
			"https://kasirpintar-beta.vercel.app/", // TODO: ganti dengan domain Vercel yang sebenarnya
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
