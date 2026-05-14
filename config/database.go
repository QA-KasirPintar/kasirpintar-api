// LOKASI: config/database.go
package config

import (
	"fmt"
	"log"
	"os"

	"kasirpintar-api/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB

func ConnectDatabase() {
	// Validasi env wajib
	requiredEnvs := []string{
		"DB_HOST",
		"DB_PORT",
		"DB_USER",
		"DB_PASSWORD",
		"DB_NAME",
	}

	for _, env := range requiredEnvs {
		if os.Getenv(env) == "" {
			log.Fatalf("ENV %s belum diset", env)
		}
	}

	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)

	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Gagal konek database: %v", err)
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
			log.Fatalf("Gagal AutoMigrate: %v", err)
		}
		log.Println("AutoMigrate selesai")
	}

	DB = database
	log.Println("Database terkoneksi dengan sukses")
}
