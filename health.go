package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/expki/ZeroLoop.git/database"
	"github.com/expki/ZeroLoop.git/logger"
	"golang.org/x/sync/singleflight"
)

var singleHealth singleflight.Group

// healthHandler returns the service health status
func healthHandler(w http.ResponseWriter, r *http.Request) {
	ch := singleHealth.DoChan("database", func() (any, error) {
		if logger.Log == nil {
			return "logger not initialized", nil
		}
		if database.DB == nil {
			return "database not initialized", nil
		}

		// database health
		db, err := database.DB.DB()
		if err != nil {
			logger.Log.Errorw("database connection failed", "error", err)
			return "database connection failed", nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err = db.PingContext(ctx)
		if err != nil {
			logger.Log.Errorw("database ping failed", "error", err)
			return "database ping failed", nil
		}

		// search health
		// TODO: implement search health

		// llm health (chat high, chat low, embed, rerank)
		// TODO: implement llm health

		return "OK", nil
	})

	var status string
	select {
	case <-r.Context().Done():
		w.WriteHeader(http.StatusRequestTimeout)
		fmt.Fprint(w, "request canceled")
		return
	case res := <-ch:
		if res.Err != nil {
			logger.Log.Errorw("catastrophic health check failure", "error", res.Err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "catastrophic health check failure")
			return
		}
		text, ok := res.Val.(string)
		if !ok {
			logger.Log.Errorw("catastrophic health check failure, value not string", "value", res.Val)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "catastrophic health check failure")
			return
		}
		status = text
	}

	if status == "OK" {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	fmt.Fprint(w, status)
	return
}
