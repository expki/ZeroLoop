package config

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/denisbrodbeck/machineid"
)

type Config struct {
	// Server
	Port        int
	Environment string
	LogLevel    string // "debug", "info", "warn", "error"

	// Database
	DBDriver    string // "sqlite" or "postgres"
	DatabaseURL string

	// Search
	SearchDir string

	// JWT
	JWTSecret        string
	JWTAccessExpiry  int // seconds
	JWTRefreshExpiry int // seconds

	// Stripe
	StripeSecretKey     string
	StripeWebhookSecret string

	// Email (SMTP)
	SMTPHost     string
	SMTPPort     int
	SMTPSecure   bool
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string

	// TLS
	TLSCert string
	TLSKey  string
}

var cfg *Config

func Load() *Config {
	if cfg != nil {
		return cfg
	}

	cfg = &Config{
		// Server
		Port:        must(getEnv("PORT", 9368)),
		Environment: must(getEnv("ENVIRONMENT", "development")),
		LogLevel:    must(getEnv("LOG_LEVEL", "")), // Empty means auto (debug for dev, info for prod)

		// Database
		DBDriver: must(getEnv("DB_DRIVER", "sqlite")),
		DatabaseURL: must(getEnv("DATABASE_URL", func() string {
			return filepath.Join(getBaseFolder(), "agentzero.db")
		}())),

		// Search
		SearchDir: must(getEnv("SEARCH_DIR", func() string {
			return filepath.Join(getBaseFolder(), "agentzero.search/")
		}())),

		// JWT
		JWTSecret: must(getEnv("JWT_SECRET", func() string {
			id, err := machineid.ProtectedID("zeroloop")
			if err == nil {
				return id
			}
			id, err = os.Hostname()
			if err != nil {
				id = "default"
			}
			mac := hmac.New(sha256.New, []byte(id))
			mac.Write([]byte("zeroloop"))
			return hex.EncodeToString(mac.Sum(nil))
		}())),
		JWTAccessExpiry:  must(getEnv("JWT_ACCESS_EXPIRY", 3600)),     // 1 hour
		JWTRefreshExpiry: must(getEnv("JWT_REFRESH_EXPIRY", 2592000)), // 30 days

		// Stripe
		StripeSecretKey:     must(getEnv("STRIPE_SECRET_KEY", "")),
		StripeWebhookSecret: must(getEnv("STRIPE_WEBHOOK_SECRET", "")),

		// Email
		SMTPHost:     must(getEnv("SMTP_HOST", "")),
		SMTPPort:     must(getEnv("SMTP_PORT", 587)),
		SMTPSecure:   must(getEnv("SMTP_SECURE", false)),
		SMTPUser:     must(getEnv("SMTP_USER", "")),
		SMTPPassword: must(getEnv("SMTP_PASSWORD", "")),
		SMTPFrom:     must(getEnv("SMTP_FROM", "noreply@mail.vdh.dev")),

		// TLS
		TLSCert: must(getEnv("TLS_CERT", "")),
		TLSKey:  must(getEnv("TLS_KEY", "")),
	}

	return cfg
}

func Get() *Config {
	if cfg == nil {
		return Load()
	}
	return cfg
}

func getEnv[T any](key string, fallback T) (T, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	var result any
	var err error

	switch any(fallback).(type) {
	case string:
		return any(value).(T), nil
	case int:
		result, err = strconv.Atoi(value)
	case bool:
		result, err = strconv.ParseBool(value)
	case float64:
		result, err = strconv.ParseFloat(value, 64)
	case float32:
		var float64Result float64
		float64Result, err = strconv.ParseFloat(value, 32)
		result = float32(float64Result)
	case time.Duration:
		result, err = time.ParseDuration(value)
	default:
		return fallback, fmt.Errorf("unknown %T type", fallback)
	}

	if err != nil {
		return fallback, fmt.Errorf("parse failure %T: %v", fallback, err)
	}
	return result.(T), nil
}

func must[T any](result T, err error) T {
	if err != nil {
		panic(err)
	}
	return result
}

func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

func (c *Config) TLSEnabled() bool {
	return c.TLSCert != "" && c.TLSKey != ""
}

func getBaseFolder() string {
	exePath, err := os.Executable()
	if err != nil {
		return getWorkingFolder()
	}
	if strings.Contains(exePath, os.TempDir()) {
		return getWorkingFolder()
	}
	return filepath.Dir(exePath)
}

func getWorkingFolder() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	return dir
}
