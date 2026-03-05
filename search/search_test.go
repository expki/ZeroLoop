package search

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blevesearch/bleve/v2"
)

func setupTestIndex(t *testing.T) func() {
	t.Helper()
	dir := filepath.Join(os.TempDir(), "zeroloop_search_test")
	os.RemoveAll(dir)

	var err error
	idx, err = bleve.New(dir, bleve.NewIndexMapping())
	if err != nil {
		t.Fatalf("failed to create test index: %v", err)
	}

	return func() {
		Close()
		idx = nil
		os.RemoveAll(dir)
	}
}

func TestSearchIndexAndSearch(t *testing.T) {
	cleanup := setupTestIndex(t)
	defer cleanup()

	doc := Document{
		ID:        "test-1",
		Content:   "The quick brown fox jumps over the lazy dog",
		Type:      "knowledge",
		Heading:   "Test Document",
		CreatedAt: time.Now(),
	}
	if err := Index(doc); err != nil {
		t.Fatal(err)
	}

	count, _ := DocCount()
	if count != 1 {
		t.Errorf("expected 1 document, got %d", count)
	}

	results, err := Search("fox", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one search result")
	}
	if results[0].ID != "test-1" {
		t.Errorf("expected id 'test-1', got '%s'", results[0].ID)
	}
}

func TestSearchNoResults(t *testing.T) {
	cleanup := setupTestIndex(t)
	defer cleanup()

	results, err := Search("nonexistent", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSearchDelete(t *testing.T) {
	cleanup := setupTestIndex(t)
	defer cleanup()

	doc := Document{
		ID:        "delete-test",
		Content:   "This document will be deleted",
		Type:      "message",
		CreatedAt: time.Now(),
	}
	if err := Index(doc); err != nil {
		t.Fatal(err)
	}

	if err := Delete("delete-test"); err != nil {
		t.Fatal(err)
	}

	count, _ := DocCount()
	if count != 0 {
		t.Errorf("expected 0 documents after delete, got %d", count)
	}
}

func TestSearchMultipleDocuments(t *testing.T) {
	cleanup := setupTestIndex(t)
	defer cleanup()

	docs := []Document{
		{ID: "doc-1", Content: "Golang is great for concurrency", Type: "knowledge"},
		{ID: "doc-2", Content: "React hooks simplify state management", Type: "knowledge"},
		{ID: "doc-3", Content: "Golang goroutines are lightweight", Type: "message"},
	}
	for _, doc := range docs {
		if err := Index(doc); err != nil {
			t.Fatal(err)
		}
	}

	results, err := Search("golang", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results for 'golang', got %d", len(results))
	}
}

func TestSearchNilIndex(t *testing.T) {
	oldIdx := idx
	idx = nil
	defer func() { idx = oldIdx }()

	_, err := Search("test", 10)
	if err == nil {
		t.Error("expected error when index is nil")
	}

	err = Index(Document{ID: "x", Content: "test"})
	if err == nil {
		t.Error("expected error when index is nil")
	}
}
