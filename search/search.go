package search

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/blevesearch/bleve/v2"

	"github.com/expki/ZeroLoop.git/config"
	"github.com/expki/ZeroLoop.git/logger"
)

var idx bleve.Index

// Init initializes the Bleve search index (call once at startup)
func Init() error {
	dir := config.Load().SearchDir
	_ = os.MkdirAll(filepath.Dir(dir), 0755)

	var err error
	idx, err = bleve.Open(dir)
	if err == nil {
		count, _ := idx.DocCount()
		logger.Log.Infow("opened existing search index", "documents", count)
		return nil
	}
	logger.Log.Debugw("creating new search index", "reason", err)

	idx, err = bleve.New(dir, bleve.NewIndexMapping())
	if err != nil {
		return fmt.Errorf("create search index: %w", err)
	}
	logger.Log.Info("created new search index")
	return nil
}

// Document represents a searchable document
type Document struct {
	ID        string    `json:"id"`
	ChatID    string    `json:"chat_id"`
	Content   string    `json:"content"`
	Type      string    `json:"type"` // "message", "memory", "knowledge"
	Heading   string    `json:"heading"`
	CreatedAt time.Time `json:"created_at"`
}

// SearchResult represents a search hit
type SearchResult struct {
	ID      string  `json:"id"`
	Score   float64 `json:"score"`
	Content string  `json:"content"`
	Type    string  `json:"type"`
	Heading string  `json:"heading"`
	ChatID  string  `json:"chat_id"`
}

// Index adds or updates a document in the search index
func Index(doc Document) error {
	if idx == nil {
		return fmt.Errorf("search index not initialized")
	}
	return idx.Index(doc.ID, doc)
}

// Search performs a full-text search and returns matching documents
func Search(query string, maxResults int) ([]SearchResult, error) {
	if idx == nil {
		return nil, fmt.Errorf("search index not initialized")
	}
	if maxResults <= 0 {
		maxResults = 10
	}

	searchReq := bleve.NewSearchRequestOptions(bleve.NewQueryStringQuery(query), maxResults, 0, false)
	searchReq.Fields = []string{"content", "type", "heading", "chat_id"}

	searchResults, err := idx.Search(searchReq)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	results := make([]SearchResult, 0, len(searchResults.Hits))
	for _, hit := range searchResults.Hits {
		r := SearchResult{
			ID:    hit.ID,
			Score: hit.Score,
		}
		if v, ok := hit.Fields["content"].(string); ok {
			r.Content = v
		}
		if v, ok := hit.Fields["type"].(string); ok {
			r.Type = v
		}
		if v, ok := hit.Fields["heading"].(string); ok {
			r.Heading = v
		}
		if v, ok := hit.Fields["chat_id"].(string); ok {
			r.ChatID = v
		}
		results = append(results, r)
	}

	return results, nil
}

// Delete removes a document from the search index
func Delete(id string) error {
	if idx == nil {
		return fmt.Errorf("search index not initialized")
	}
	return idx.Delete(id)
}

// DocCount returns the number of indexed documents
func DocCount() (uint64, error) {
	if idx == nil {
		return 0, fmt.Errorf("search index not initialized")
	}
	return idx.DocCount()
}

// Close closes the search index
func Close() error {
	if idx == nil {
		return nil
	}
	return idx.Close()
}
