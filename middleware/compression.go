package middleware

import (
	"compress/gzip"
	"net/http"
	"strings"
	"sync"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

// Pool of zstd encoders for reuse
var zstdEncoderPool = sync.Pool{
	New: func() any {
		encoder, _ := zstd.NewWriter(nil,
			zstd.WithEncoderLevel(zstd.SpeedDefault),
			zstd.WithEncoderCRC(false),
			zstd.WithEncoderConcurrency(1),
		)
		return encoder
	},
}

// Pool of brotli writers for reuse
var brotliWriterPool = sync.Pool{
	New: func() any {
		return brotli.NewWriterLevel(nil, brotli.DefaultCompression)
	},
}

// Pool of gzip writers for reuse
var gzipWriterPool = sync.Pool{
	New: func() any {
		w, _ := gzip.NewWriterLevel(nil, gzip.DefaultCompression)
		return w
	},
}

// zstdResponseWriter wraps http.ResponseWriter to compress responses
type zstdResponseWriter struct {
	http.ResponseWriter
	encoder *zstd.Encoder
}

func (w *zstdResponseWriter) Write(b []byte) (int, error) {
	return w.encoder.Write(b)
}

// brotliResponseWriter wraps http.ResponseWriter to compress responses
type brotliResponseWriter struct {
	http.ResponseWriter
	writer *brotli.Writer
}

func (w *brotliResponseWriter) Write(b []byte) (int, error) {
	return w.writer.Write(b)
}

// gzipResponseWriter wraps http.ResponseWriter to compress responses
type gzipResponseWriter struct {
	http.ResponseWriter
	writer *gzip.Writer
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.writer.Write(b)
}

// CompressionMiddleware selects the best compression based on Accept-Encoding.
// Server preference order: zstd > br > gzip
func CompressionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip compression for WebSocket upgrades
		if r.Header.Get("Upgrade") == "websocket" {
			next.ServeHTTP(w, r)
			return
		}

		accept := r.Header.Get("Accept-Encoding")
		w.Header().Add("Vary", "Accept-Encoding")

		// Server preference: zstd > br > gzip
		switch {
		case strings.Contains(accept, "zstd"):
			serveZstd(w, r, next)
		case strings.Contains(accept, "br"):
			serveBrotli(w, r, next)
		case strings.Contains(accept, "gzip"):
			serveGzip(w, r, next)
		default:
			next.ServeHTTP(w, r)
		}
	})
}

func serveZstd(w http.ResponseWriter, r *http.Request, next http.Handler) {
	encoder := zstdEncoderPool.Get().(*zstd.Encoder)
	encoder.Reset(w)
	defer func() {
		encoder.Close()
		zstdEncoderPool.Put(encoder)
	}()

	w.Header().Set("Content-Encoding", "zstd")
	w.Header().Del("Content-Length")

	next.ServeHTTP(&zstdResponseWriter{ResponseWriter: w, encoder: encoder}, r)
}

func serveBrotli(w http.ResponseWriter, r *http.Request, next http.Handler) {
	writer := brotliWriterPool.Get().(*brotli.Writer)
	writer.Reset(w)
	defer func() {
		writer.Close()
		brotliWriterPool.Put(writer)
	}()

	w.Header().Set("Content-Encoding", "br")
	w.Header().Del("Content-Length")

	next.ServeHTTP(&brotliResponseWriter{ResponseWriter: w, writer: writer}, r)
}

func serveGzip(w http.ResponseWriter, r *http.Request, next http.Handler) {
	writer := gzipWriterPool.Get().(*gzip.Writer)
	writer.Reset(w)
	defer func() {
		writer.Close()
		gzipWriterPool.Put(writer)
	}()

	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Del("Content-Length")

	next.ServeHTTP(&gzipResponseWriter{ResponseWriter: w, writer: writer}, r)
}
