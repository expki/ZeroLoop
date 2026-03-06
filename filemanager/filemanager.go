package filemanager

import (
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
)

// FileInfo represents metadata about a file in a project.
type FileInfo struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	IsDir    bool   `json:"is_dir"`
	Size     int64  `json:"size"`
	MimeType string `json:"mime_type"`
}

// FileManager handles filesystem operations for project files with path validation and locking.
type FileManager struct {
	baseDir string
	Locks   *FileLockManager
}

// New creates a new FileManager rooted at the given base directory.
func New(baseDir string) *FileManager {
	return &FileManager{
		baseDir: baseDir,
		Locks:   NewFileLockManager(),
	}
}

// ProjectDir returns the absolute path for a project's directory.
func (fm *FileManager) ProjectDir(projectID string) string {
	return filepath.Join(fm.baseDir, projectID)
}

// ValidatePath validates and resolves a relative path within a project directory.
// Returns the absolute path if valid, or an error if the path escapes the project root.
func (fm *FileManager) ValidatePath(projectID, relPath string) (string, error) {
	if relPath == "" {
		return "", fmt.Errorf("empty path")
	}

	// Reject absolute paths
	if filepath.IsAbs(relPath) {
		return "", fmt.Errorf("absolute paths not allowed")
	}

	// Clean the path and check for traversal
	cleaned := filepath.Clean(relPath)
	if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, string(filepath.Separator)+"..") {
		return "", fmt.Errorf("path traversal not allowed")
	}

	// Reject "." — operating on the project root itself is not allowed
	if cleaned == "." {
		return "", fmt.Errorf("cannot operate on project root")
	}

	projectDir := fm.ProjectDir(projectID)
	absPath := filepath.Join(projectDir, cleaned)

	// Double-check the resolved path is within the project directory
	rel, err := filepath.Rel(projectDir, absPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path escapes project directory")
	}

	return absPath, nil
}

// CreateProject creates the project directory on disk.
func (fm *FileManager) CreateProject(projectID string) error {
	return os.MkdirAll(fm.ProjectDir(projectID), 0755)
}

// DeleteProject removes the project directory and all its contents.
func (fm *FileManager) DeleteProject(projectID string) error {
	dir := fm.ProjectDir(projectID)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}
	return os.RemoveAll(dir)
}

// ReadFile reads a file from the project, acquiring a read lock.
func (fm *FileManager) ReadFile(projectID, relPath string) ([]byte, error) {
	absPath, err := fm.ValidatePath(projectID, relPath)
	if err != nil {
		return nil, err
	}

	fm.Locks.RLock(absPath)
	defer fm.Locks.RUnlock(absPath)

	return os.ReadFile(absPath)
}

// WriteFile writes content to a file in the project, acquiring a write lock.
// Creates parent directories as needed.
func (fm *FileManager) WriteFile(projectID, relPath string, content []byte) error {
	absPath, err := fm.ValidatePath(projectID, relPath)
	if err != nil {
		return err
	}

	fm.Locks.Lock(absPath)
	defer fm.Locks.Unlock(absPath)

	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(absPath, content, 0644)
}

// WriteFileFromReader writes content from a reader to a file, acquiring a write lock.
// Used for multipart uploads to avoid buffering entire files in memory.
func (fm *FileManager) WriteFileFromReader(projectID, relPath string, reader io.Reader) (int64, error) {
	absPath, err := fm.ValidatePath(projectID, relPath)
	if err != nil {
		return 0, err
	}

	fm.Locks.Lock(absPath)
	defer fm.Locks.Unlock(absPath)

	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return 0, err
	}

	f, err := os.Create(absPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	return io.Copy(f, reader)
}

// DeleteFile removes a file from the project, acquiring a write lock.
func (fm *FileManager) DeleteFile(projectID, relPath string) error {
	absPath, err := fm.ValidatePath(projectID, relPath)
	if err != nil {
		return err
	}

	fm.Locks.Lock(absPath)
	defer fm.Locks.Unlock(absPath)

	return os.Remove(absPath)
}

// CreateDir creates a directory within the project.
func (fm *FileManager) CreateDir(projectID, relPath string) error {
	absPath, err := fm.ValidatePath(projectID, relPath)
	if err != nil {
		return err
	}
	return os.MkdirAll(absPath, 0755)
}

// RenameFile moves/renames a file or directory within a project.
// Both oldPath and newPath are relative to the project root.
func (fm *FileManager) RenameFile(projectID, oldRelPath, newRelPath string) error {
	oldAbs, err := fm.ValidatePath(projectID, oldRelPath)
	if err != nil {
		return fmt.Errorf("invalid source path: %w", err)
	}
	newAbs, err := fm.ValidatePath(projectID, newRelPath)
	if err != nil {
		return fmt.Errorf("invalid destination path: %w", err)
	}

	// Check source exists
	if _, err := os.Stat(oldAbs); os.IsNotExist(err) {
		return fmt.Errorf("source does not exist")
	}

	// Prevent moving a directory into a subdirectory of itself
	if strings.HasPrefix(newAbs+string(filepath.Separator), oldAbs+string(filepath.Separator)) {
		return fmt.Errorf("cannot move a directory into itself")
	}

	// Check destination does not already exist
	if _, err := os.Stat(newAbs); err == nil {
		return fmt.Errorf("destination already exists")
	}

	// Ensure destination parent directory exists
	if err := os.MkdirAll(filepath.Dir(newAbs), 0755); err != nil {
		return err
	}

	// Lock both paths (alphabetical order to prevent deadlocks)
	first, second := oldAbs, newAbs
	if newAbs < oldAbs {
		first, second = newAbs, oldAbs
	}
	fm.Locks.Lock(first)
	defer fm.Locks.Unlock(first)
	fm.Locks.Lock(second)
	defer fm.Locks.Unlock(second)

	return os.Rename(oldAbs, newAbs)
}

// DeleteFileRecursive removes a file or directory (and all contents) from the project.
func (fm *FileManager) DeleteFileRecursive(projectID, relPath string) error {
	absPath, err := fm.ValidatePath(projectID, relPath)
	if err != nil {
		return err
	}
	fm.Locks.Lock(absPath)
	defer fm.Locks.Unlock(absPath)
	return os.RemoveAll(absPath)
}

// ListFiles returns metadata for all files in the project directory (recursive).
func (fm *FileManager) ListFiles(projectID string) ([]FileInfo, error) {
	projectDir := fm.ProjectDir(projectID)
	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		return []FileInfo{}, nil
	}

	var files []FileInfo
	err := filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		// Skip the root project directory itself
		if path == projectDir {
			return nil
		}

		relPath, _ := filepath.Rel(projectDir, path)
		mimeType := ""
		if !info.IsDir() {
			mimeType = mime.TypeByExtension(filepath.Ext(info.Name()))
		}

		files = append(files, FileInfo{
			Path:     relPath,
			Name:     info.Name(),
			IsDir:    info.IsDir(),
			Size:     info.Size(),
			MimeType: mimeType,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}
