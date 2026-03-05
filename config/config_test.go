package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	// Reset singleton for test
	cfg = nil

	c := Load()
	if c.Port != 9368 {
		t.Errorf("expected default port 9368, got %d", c.Port)
	}
	if c.Environment != "development" {
		t.Errorf("expected development, got %s", c.Environment)
	}
	if c.DBDriver != "sqlite" {
		t.Errorf("expected sqlite, got %s", c.DBDriver)
	}
	if c.LLMBaseURL != "http://192.168.10.15:8081" {
		t.Errorf("expected default LLM URL, got %s", c.LLMBaseURL)
	}
	// Reset for other tests
	cfg = nil
}

func TestLoadWithEnv(t *testing.T) {
	cfg = nil
	os.Setenv("PORT", "3000")
	os.Setenv("ENVIRONMENT", "production")
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("ENVIRONMENT")
		cfg = nil
	}()

	c := Load()
	if c.Port != 3000 {
		t.Errorf("expected port 3000, got %d", c.Port)
	}
	if c.Environment != "production" {
		t.Errorf("expected production, got %s", c.Environment)
	}
}

func TestIsDevelopment(t *testing.T) {
	cfg = nil
	c := Load()
	if !c.IsDevelopment() {
		t.Error("expected IsDevelopment() = true")
	}
	if c.IsProduction() {
		t.Error("expected IsProduction() = false")
	}
	cfg = nil
}

func TestIsProduction(t *testing.T) {
	cfg = nil
	os.Setenv("ENVIRONMENT", "production")
	defer func() {
		os.Unsetenv("ENVIRONMENT")
		cfg = nil
	}()

	c := Load()
	if c.IsProduction() != true {
		t.Error("expected IsProduction() = true")
	}
	if c.IsDevelopment() != false {
		t.Error("expected IsDevelopment() = false")
	}
}

func TestTLSEnabled(t *testing.T) {
	cfg = nil
	c := Load()
	if c.TLSEnabled() {
		t.Error("expected TLSEnabled() = false with no certs")
	}
	cfg = nil
}

func TestTLSEnabledWithCerts(t *testing.T) {
	cfg = nil
	os.Setenv("TLS_CERT", "/path/to/cert.pem")
	os.Setenv("TLS_KEY", "/path/to/key.pem")
	defer func() {
		os.Unsetenv("TLS_CERT")
		os.Unsetenv("TLS_KEY")
		cfg = nil
	}()

	c := Load()
	if !c.TLSEnabled() {
		t.Error("expected TLSEnabled() = true with certs set")
	}
}

func TestGet(t *testing.T) {
	cfg = nil
	c1 := Get()
	c2 := Get()
	if c1 != c2 {
		t.Error("expected Get() to return same singleton")
	}
	cfg = nil
}

func TestLoadSingleton(t *testing.T) {
	cfg = nil
	c1 := Load()
	c2 := Load()
	if c1 != c2 {
		t.Error("expected Load() to return same singleton on second call")
	}
	cfg = nil
}

func TestDefaultJWTExpiry(t *testing.T) {
	cfg = nil
	c := Load()
	if c.JWTAccessExpiry != 3600 {
		t.Errorf("expected JWT access expiry 3600, got %d", c.JWTAccessExpiry)
	}
	if c.JWTRefreshExpiry != 2592000 {
		t.Errorf("expected JWT refresh expiry 2592000, got %d", c.JWTRefreshExpiry)
	}
	cfg = nil
}

func TestDefaultSMTPPort(t *testing.T) {
	cfg = nil
	c := Load()
	if c.SMTPPort != 587 {
		t.Errorf("expected SMTP port 587, got %d", c.SMTPPort)
	}
	cfg = nil
}
