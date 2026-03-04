package search

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/blevesearch/bleve/v2"

	"github.com/expki/ZeroLoop.git/config"
	"github.com/expki/ZeroLoop.git/logger"
)

var (
	once sync.Once
)

var idx = func() (idx bleve.Index) {
	var err error
	_ = os.MkdirAll(filepath.Dir(config.Load().SearchDir), 0755)

	// Existing search index
	idx, err = bleve.Open(config.Load().SearchDir)
	if err == nil {
		count, _ := idx.DocCount()
		logger.Log.Infow("opened existing search", "documents", count)
		return
	}
	logger.Log.Debugw("opening existing search", "error", err)

	// New search index
	idx, err = bleve.New(config.Load().SearchDir, bleve.NewIndexMapping())
	if err != nil {
		logger.Log.Fatalw("creating new search", "error", err)
		return
	}

	return idx
}
