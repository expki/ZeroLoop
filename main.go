package main

import (
	"context"
	"crypto/tls"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"

	"github.com/expki/ZeroLoop.git/api"
	"github.com/expki/ZeroLoop.git/config"
	"github.com/expki/ZeroLoop.git/database"
	"github.com/expki/ZeroLoop.git/filemanager"
	"github.com/expki/ZeroLoop.git/llm"
	"github.com/expki/ZeroLoop.git/logger"
	"github.com/expki/ZeroLoop.git/models"
	"github.com/expki/ZeroLoop.git/middleware"
	"github.com/expki/ZeroLoop.git/search"
)

func main() {
	appCtx, cancelApp := context.WithCancel(context.Background())
	defer cancelApp()

	// Load .env file if it exists
	_ = godotenv.Load()

	// Load configuration
	cfg := config.Load()

	// Initialize logger
	logger.Init(cfg.IsDevelopment(), cfg.LogLevel)
	defer logger.Sync()

	// Connect to database
	if err := database.Connect(); err != nil {
		logger.Log.Fatalw("failed to connect to database", "error", err)
	}

	// Run migrations
	if err := database.AutoMigrate(); err != nil {
		logger.Log.Fatalw("failed to run migrations", "error", err)
	}

	// Ensure projects directory exists
	if err := os.MkdirAll(cfg.ProjectsDir, 0755); err != nil {
		logger.Log.Fatalw("failed to create projects directory", "error", err, "path", cfg.ProjectsDir)
	}
	logger.Log.Infow("projects directory ready", "path", cfg.ProjectsDir)

	// Initialize search index
	if err := search.Init(); err != nil {
		logger.Log.Fatalw("failed to initialize search index", "error", err)
	}
	defer search.Close()

	// Initialize file manager
	fm := filemanager.New(cfg.ProjectsDir)
	fm.Resolver = func(projectID string) string {
		var p models.Project
		if err := database.Get().Select("name").First(&p, "id = ?", projectID).Error; err == nil {
			return p.Name
		}
		return ""
	}
	logger.Log.Infow("file manager initialized", "projects_dir", cfg.ProjectsDir)

	// Initialize LLM client
	llmClient := llm.NewClient(cfg.LLMBaseURL)
	logger.Log.Infow("LLM client initialized", "url", cfg.LLMBaseURL)

	// Initialize WebSocket hub
	hub := api.NewHub(llmClient, fm)
	go hub.Run()

	// Create mux and register routes
	mux := http.NewServeMux()

	// API routes
	api.RegisterRoutes(mux, hub, fm)

	// WebSocket endpoint
	mux.HandleFunc("/ws", hub.HandleWebSocket)

	// Health check
	mux.HandleFunc("/health", healthHandler)

	// SEO endpoints (dynamic robots.txt, llms.txt, sitemap.xml)
	mux.HandleFunc("/robots.txt", createRobotsTxtHandler())
	mux.HandleFunc("/llms.txt", createLlmsTxtHandler())
	mux.HandleFunc("/sitemap.xml", createSitemapXmlHandler())

	// Serve embedded frontend static files
	logger.Log.Info("serving embedded frontend")
	staticUncompressed := http.FileServerFS(dist)
	staticZstd := http.FileServerFS(distZstd)
	staticBrotli := http.FileServerFS(distBrotli)
	staticGzip := http.FileServerFS(distGzip)
	mux.Handle("/", func() http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip API routes and SEO endpoints (handled separately)
			if strings.HasPrefix(r.URL.Path, "/api/") ||
				strings.HasPrefix(r.URL.Path, "/ws") ||
				strings.HasPrefix(r.URL.Path, "/health") ||
				strings.HasPrefix(r.URL.Path, "/robots.txt") ||
				strings.HasPrefix(r.URL.Path, "/llms.txt") ||
				strings.HasPrefix(r.URL.Path, "/sitemap.xml") {
				http.NotFound(w, r)
				return
			}

			// Try to open the file in embedded FS
			path := strings.TrimPrefix(r.URL.Path, "/")
			if path == "" {
				path = "index.html"
			}

			// Handle routes
			_, err := fs.Stat(dist, path)
			if err != nil && r.URL.Path != "/" {
				// Check if it's a file request (has extension)
				if !strings.Contains(path, ".") {
					// No extension, probably a route - serve index.html (SPA fallback)
					r.URL.Path = "/"
				}
			}

			// Select pre-compressed static files based on Accept-Encoding
			// Server preference: zstd > br > gzip
			accept := r.Header.Get("Accept-Encoding")
			var static http.Handler
			switch {
			case strings.Contains(accept, "zstd"):
				// Set Content-Type from extension before serving compressed content
				if ct := mime.TypeByExtension(filepath.Ext(r.URL.Path)); ct != "" {
					w.Header().Set("Content-Type", ct)
				}
				w.Header().Set("Content-Encoding", "zstd")
				static = staticZstd
			case strings.Contains(accept, "br"):
				if ct := mime.TypeByExtension(filepath.Ext(r.URL.Path)); ct != "" {
					w.Header().Set("Content-Type", ct)
				}
				w.Header().Set("Content-Encoding", "br")
				static = staticBrotli
			case strings.Contains(accept, "gzip"):
				if ct := mime.TypeByExtension(filepath.Ext(r.URL.Path)); ct != "" {
					w.Header().Set("Content-Type", ct)
				}
				w.Header().Set("Content-Encoding", "gzip")
				static = staticGzip
			default:
				static = staticUncompressed
			}
			static.ServeHTTP(w, r)
		})
	}())

	// Security middleware chain (applied to ALL responses)
	// Order: RateLimiter -> WAF -> SecurityHeaders -> CacheHeaders -> mux -> AV (optional) -> handlers
	securityCfg := middleware.SecurityConfig{
		IsDevelopment: cfg.IsDevelopment(),
		TLSEnabled:    cfg.TLSEnabled(),
	}
	handler := middleware.CacheMiddleware()(mux)
	handler = securityCfg.SecurityHeaders()(handler)

	// Rate Limiting
	handler = middleware.NewRateLimiter(handler)

	// Start server
	serverAddr := net.JoinHostPort("", strconv.Itoa(cfg.Port))
	logger.Log.Infow("server listening", "address", serverAddr)

	if cfg.TLSEnabled() {
		// Create certificate manager for automatic reload
		certManager, err := NewCertManager(appCtx, cfg.TLSCert, cfg.TLSKey)
		if err != nil {
			logger.Log.Fatalw("failed to load TLS certificate", "error", err)
		}

		server := &http.Server{
			Addr:    serverAddr,
			Handler: handler,
			TLSConfig: &tls.Config{
				GetCertificate: certManager.GetCertificate,
				ClientAuth:     tls.NoClientCert,
			},
		}

		logger.Log.Infow("server started", "url", "https://localhost:"+strconv.Itoa(cfg.Port))
		if err := server.ListenAndServeTLS("", ""); err != nil {
			logger.Log.Fatalw("server failed", "error", err)
		}
	} else {
		logger.Log.Infow("server started", "url", "http://localhost:"+strconv.Itoa(cfg.Port))
		if err := http.ListenAndServe(serverAddr, handler); err != nil {
			logger.Log.Fatalw("server failed", "error", err)
		}
	}
}
