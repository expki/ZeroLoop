package api

import (
	"net/http"
	"strconv"

	"github.com/expki/ZeroLoop.git/filemanager"
)

func searchProjectFiles(w http.ResponseWriter, r *http.Request, fm *filemanager.FileManager) {
	projectID := r.PathValue("id")
	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	maxResults := 100
	if m := r.URL.Query().Get("max"); m != "" {
		if v, err := strconv.Atoi(m); err == nil && v > 0 {
			maxResults = v
		}
	}

	results, err := fm.SearchFiles(projectID, query, maxResults)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search failed: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, results)
}
