package config

import (
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func ConnectDB() {
	// Load environment variables from .env
	if err := godotenv.Load(); err != nil {
		log.Println("File .env tidak ditemukan, menggunakan environment variables")
	}

	// Get Supabase connection string
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL belum diatur di file .env")
	}

	// Connect using GORM with PostgreSQL driver
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("Gagal terhubung ke database Supabase: %v", err)
	}

	// Configure underlying sql.DB connection pool
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("Gagal mengambil instance database: %v", err)
	}

	sqlDB.SetMaxOpenConns(5)
	sqlDB.SetMaxIdleConns(2)
	sqlDB.SetConnMaxLifetime(15 * time.Minute)

	// Test the connection
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		log.Fatalf("Gagal melakukan ping ke Supabase database: %v", err)
	}

	DB = db
	log.Println("Successfully connected to Supabase PostgreSQL using GORM!")
}

// CloseDB closes the database connection pool.
func CloseDB() {
	if DB != nil {
		sqlDB, err := DB.DB()
		if err == nil && sqlDB != nil {
			sqlDB.Close()
		}
	}
}
