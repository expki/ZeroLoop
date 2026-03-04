package main

import (
	"context"
	"crypto/tls"
	"sync"
	"time"

	"github.com/expki/ZeroLoop.git/logger"
)

// CertManager handles TLS certificate loading and automatic reloading
type CertManager struct {
	certPath string
	keyPath  string
	cert     *tls.Certificate
	mu       sync.RWMutex
}

// NewCertManager creates a new certificate manager that reloads certificates periodically
func NewCertManager(appCtx context.Context, certPath, keyPath string) (*CertManager, error) {
	cm := &CertManager{
		certPath: certPath,
		keyPath:  keyPath,
	}

	// Load certificate initially
	if err := cm.reload(); err != nil {
		return nil, err
	}

	// Start background reload goroutine
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-appCtx.Done():
				return
			case <-ticker.C:
				if err := cm.reload(); err != nil {
					logger.Log.Errorw("failed to reload TLS certificate", "error", err)
				}
			}
		}
	}()

	return cm, nil
}

// reload loads the certificate from disk
func (cm *CertManager) reload() error {
	cert, err := tls.LoadX509KeyPair(cm.certPath, cm.keyPath)
	if err != nil {
		return err
	}

	cm.mu.Lock()
	cm.cert = &cert
	cm.mu.Unlock()

	logger.Log.Infow("TLS certificate loaded", "path", cm.certPath)
	return nil
}

// GetCertificate returns the cached certificate for use with tls.Config
func (cm *CertManager) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.cert, nil
}
