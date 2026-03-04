package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/expki/ZeroLoop.git/models"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/expki/ZeroLoop.git/config"
	"github.com/expki/ZeroLoop.git/logger"
)

var DB *gorm.DB

func Connect() error {
	cfg := config.Get()

	// Configure GORM logger
	gormLogger := gormlogger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		gormlogger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  gormlogger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)

	if cfg.IsDevelopment() {
		gormLogger = gormlogger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			gormlogger.Config{
				SlowThreshold:             time.Second,
				LogLevel:                  gormlogger.Info,
				IgnoreRecordNotFoundError: false,
				Colorful:                  true,
			},
		)
	}

	var db *gorm.DB
	var err error

	// Connect based on driver
	switch cfg.DBDriver {
	case "postgres", "postgresql":
		logger.Log.Info("connecting to PostgreSQL database")
		db, err = gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{
			Logger: gormLogger,
		})
	case "sqlite", "sqlite3":
		logger.Log.Infow("connecting to SQLite database", "path", cfg.DatabaseURL)
		db, err = gorm.Open(sqlite.Open(cfg.DatabaseURL), &gorm.Config{
			Logger: gormLogger,
		})
	default:
		return fmt.Errorf("unsupported database driver: %s (use 'sqlite' or 'postgres')", cfg.DBDriver)
	}

	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool (only for non-SQLite)
	if cfg.DBDriver != "sqlite" && cfg.DBDriver != "sqlite3" {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}

		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetMaxOpenConns(100)
		sqlDB.SetConnMaxLifetime(time.Hour)
	}

	DB = db
	logger.Log.Infow("database connected successfully", "driver", cfg.DBDriver)

	return nil
}

func AutoMigrate() error {
	logger.Log.Info("running database migrations")

	err := DB.AutoMigrate(
		&models.Example{},
	)
	if err != nil {
		return err
	}

	logger.Log.Info("database migrations completed")
	return nil
}

func Get() *gorm.DB {
	return DB
}
