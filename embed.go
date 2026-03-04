package main

import (
	"bytes"
	"compress/gzip"
	"embed"
	"fmt"
	"io"
	"io/fs"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
	"github.com/spf13/afero"
)

//go:embed dist/*
var root embed.FS

var (
	dist       fs.FS
	distZstd   fs.FS
	distBrotli fs.FS
	distGzip   fs.FS
)

func init() {
	var err error
	dist, err = fs.Sub(root, "dist")
	if err != nil {
		dist = &fsWrapper{Fs: afero.NewMemMapFs()}
	}

	memZstd := afero.NewMemMapFs()
	memBrotli := afero.NewMemMapFs()
	memGzip := afero.NewMemMapFs()

	zstdEncoder, err := zstd.NewWriter(nil,
		zstd.WithEncoderLevel(zstd.SpeedDefault),
		zstd.WithEncoderCRC(false),
		zstd.WithEncoderConcurrency(1),
	)
	if err != nil {
		panic(fmt.Sprintf("initialize zstd encoder: %v", err))
	}

	err = fs.WalkDir(dist, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		file, err := dist.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		info, err := file.Stat()
		if err != nil {
			return err
		}

		content, err := io.ReadAll(file)
		if err != nil {
			return err
		}

		perm := info.Mode().Perm()

		// Zstd
		var zstdContent []byte
		zstdContent = zstdEncoder.EncodeAll(content, zstdContent)
		if err := afero.WriteFile(memZstd, path, zstdContent, perm); err != nil {
			return err
		}

		// Brotli
		var brBuf bytes.Buffer
		brWriter := brotli.NewWriterLevel(&brBuf, brotli.DefaultCompression)
		brWriter.Write(content)
		brWriter.Close()
		if err := afero.WriteFile(memBrotli, path, brBuf.Bytes(), perm); err != nil {
			return err
		}

		// Gzip
		var gzBuf bytes.Buffer
		gzWriter, _ := gzip.NewWriterLevel(&gzBuf, gzip.DefaultCompression)
		gzWriter.Write(content)
		gzWriter.Close()
		if err := afero.WriteFile(memGzip, path, gzBuf.Bytes(), perm); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		panic(fmt.Sprintf("compress embedded dist: %v", err))
	}

	distZstd = &fsWrapper{Fs: memZstd}
	distBrotli = &fsWrapper{Fs: memBrotli}
	distGzip = &fsWrapper{Fs: memGzip}
}

type fsWrapper struct {
	afero.Fs
}

func (c *fsWrapper) Open(name string) (fs.File, error) {
	return c.Fs.Open(name)
}
