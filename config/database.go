// LOKASI: config/database.go
package config

import (
	"fmt"
	"log"
	"os"
	"time"

	"kasirpintar-api/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func ConnectDatabase() {
	// Prefer DATABASE_URL (Neon/Vercel convention)
	dsn := os.Getenv("DATABASE_URL")

	if dsn == "" {
		// Fallback ke individual env vars (support both PG* and DB_* naming)
		host := os.Getenv("PGHOST")
		if host == "" {
			host = os.Getenv("DB_HOST")
		}
		port := os.Getenv("PGPORT")
		if port == "" {
			port = os.Getenv("DB_PORT")
		}
		if port == "" {
			port = "5432"
		}
		user := os.Getenv("PGUSER")
		if user == "" {
			user = os.Getenv("DB_USER")
		}
		password := os.Getenv("PGPASSWORD")
		if password == "" {
			password = os.Getenv("DB_PASSWORD")
		}
		dbname := os.Getenv("PGDATABASE")
		if dbname == "" {
			dbname = os.Getenv("DB_NAME")
		}

		if host == "" || user == "" || dbname == "" {
			log.Println("WARNING: Database env vars not fully set")
			return
		}

		dsn = fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=require",
			host, port, user, password, dbname,
		)
	}

	database, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Printf("Gagal konek database: %v", err)
		return
	}

	// Connection pool settings for serverless
	sqlDB, err := database.DB()
	if err == nil {
		sqlDB.SetMaxIdleConns(2)
		sqlDB.SetMaxOpenConns(5)
		sqlDB.SetConnMaxLifetime(5 * time.Minute)
	}

	// AutoMigrate hanya jika diizinkan
	if os.Getenv("DB_AUTOMIGRATE") == "true" {
		err = database.AutoMigrate(
			&models.TaxRate{},
			&models.Outlet{},
			&models.User{},
			&models.Menu{},
			&models.Transaction{},
			&models.TransactionDetail{},
			&models.Customer{},
			&models.Promotion{},
			&models.PasswordReset{},
			&models.PromotionCustomerSegment{},
			&models.Voucher{},
		)
		if err != nil {
			log.Printf("Gagal AutoMigrate: %v", err)
		} else {
			log.Println("AutoMigrate selesai")
		}
	}

	DB = database
	log.Println("Database terkoneksi dengan sukses (Postgres/Neon)")
}
