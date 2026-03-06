package filemanager

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestFM(t *testing.T) (*FileManager, string) {
	t.Helper()
	dir := t.TempDir()
	fm := New(dir)
	return fm, "test-project"
}

func TestValidatePath_DotPath(t *testing.T) {
	fm, projectID := setupTestFM(t)
	_ = fm.CreateProject(projectID)

	_, err := fm.ValidatePath(projectID, ".")
	if err == nil {
		t.Fatal("expected error for '.' path, got nil")
	}
	if err.Error() != "cannot operate on project root" {
		t.Fatalf("unexpected error: %s", err.Error())
	}
}

func TestValidatePath_Traversal(t *testing.T) {
	fm, projectID := setupTestFM(t)
	_ = fm.CreateProject(projectID)

	_, err := fm.ValidatePath(projectID, "../escape")
	if err == nil {
		t.Fatal("expected error for traversal path")
	}
}

func TestRenameFile_Basic(t *testing.T) {
	fm, projectID := setupTestFM(t)
	_ = fm.CreateProject(projectID)
	_ = fm.WriteFile(projectID, "old.txt", []byte("hello"))

	err := fm.RenameFile(projectID, "old.txt", "new.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Old should not exist
	oldPath := filepath.Join(fm.ProjectDir(projectID), "old.txt")
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatal("old file should not exist")
	}

	// New should exist
	content, err := fm.ReadFile(projectID, "new.txt")
	if err != nil {
		t.Fatalf("failed to read new file: %v", err)
	}
	if string(content) != "hello" {
		t.Fatalf("expected 'hello', got '%s'", string(content))
	}
}

func TestRenameFile_Directory(t *testing.T) {
	fm, projectID := setupTestFM(t)
	_ = fm.CreateProject(projectID)
	_ = fm.WriteFile(projectID, "mydir/file1.txt", []byte("one"))
	_ = fm.WriteFile(projectID, "mydir/sub/file2.txt", []byte("two"))

	err := fm.RenameFile(projectID, "mydir", "renamed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := fm.ReadFile(projectID, "renamed/file1.txt")
	if err != nil {
		t.Fatalf("failed to read renamed/file1.txt: %v", err)
	}
	if string(content) != "one" {
		t.Fatalf("expected 'one', got '%s'", string(content))
	}

	content, err = fm.ReadFile(projectID, "renamed/sub/file2.txt")
	if err != nil {
		t.Fatalf("failed to read renamed/sub/file2.txt: %v", err)
	}
	if string(content) != "two" {
		t.Fatalf("expected 'two', got '%s'", string(content))
	}
}

func TestRenameFile_PathTraversal(t *testing.T) {
	fm, projectID := setupTestFM(t)
	_ = fm.CreateProject(projectID)
	_ = fm.WriteFile(projectID, "file.txt", []byte("data"))

	err := fm.RenameFile(projectID, "file.txt", "../../escaped.txt")
	if err == nil {
		t.Fatal("expected error for traversal destination")
	}

	err = fm.RenameFile(projectID, "../../etc/passwd", "stolen.txt")
	if err == nil {
		t.Fatal("expected error for traversal source")
	}
}

func TestRenameFile_DestinationExists(t *testing.T) {
	fm, projectID := setupTestFM(t)
	_ = fm.CreateProject(projectID)
	_ = fm.WriteFile(projectID, "a.txt", []byte("a"))
	_ = fm.WriteFile(projectID, "b.txt", []byte("b"))

	err := fm.RenameFile(projectID, "a.txt", "b.txt")
	if err == nil {
		t.Fatal("expected error when destination exists")
	}
	if err.Error() != "destination already exists" {
		t.Fatalf("unexpected error: %s", err.Error())
	}
}

func TestRenameFile_SelfSubdir(t *testing.T) {
	fm, projectID := setupTestFM(t)
	_ = fm.CreateProject(projectID)
	_ = fm.CreateDir(projectID, "foo")
	_ = fm.WriteFile(projectID, "foo/file.txt", []byte("data"))

	err := fm.RenameFile(projectID, "foo", "foo/bar/foo")
	if err == nil {
		t.Fatal("expected error when moving directory into itself")
	}
	if err.Error() != "cannot move a directory into itself" {
		t.Fatalf("unexpected error: %s", err.Error())
	}
}

func TestDeleteFileRecursive(t *testing.T) {
	fm, projectID := setupTestFM(t)
	_ = fm.CreateProject(projectID)
	_ = fm.WriteFile(projectID, "dir/a.txt", []byte("a"))
	_ = fm.WriteFile(projectID, "dir/b.txt", []byte("b"))
	_ = fm.WriteFile(projectID, "dir/sub/c.txt", []byte("c"))

	err := fm.DeleteFileRecursive(projectID, "dir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dirPath := filepath.Join(fm.ProjectDir(projectID), "dir")
	if _, err := os.Stat(dirPath); !os.IsNotExist(err) {
		t.Fatal("directory should not exist after recursive delete")
	}
}

func TestDeleteFileRecursive_DotPath(t *testing.T) {
	fm, projectID := setupTestFM(t)
	_ = fm.CreateProject(projectID)
	_ = fm.WriteFile(projectID, "keep.txt", []byte("keep"))

	err := fm.DeleteFileRecursive(projectID, ".")
	if err == nil {
		t.Fatal("expected error for '.' path")
	}

	// Verify project dir still exists
	if _, err := os.Stat(fm.ProjectDir(projectID)); os.IsNotExist(err) {
		t.Fatal("project directory should still exist")
	}
}
