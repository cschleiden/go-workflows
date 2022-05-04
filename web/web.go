package web

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/internal/core"
)

type WebBackend interface {
	backend.Backend

	// TODO: Accept state?
	GetWorkflowInstances(ctx context.Context) ([]*core.WorkflowInstance, error)
}

//go:embed app/build
var embeddedFiles embed.FS

func NewMux(backend WebBackend) *http.ServeMux {
	mux := http.NewServeMux()

	// API
	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		instances, err := backend.GetWorkflowInstances(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := json.NewEncoder(w).Encode(instances); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})

	// App
	mux.Handle("/", http.FileServer(getFileSystem()))

	return mux
}

func getFileSystem() http.FileSystem {
	// Get the build subdirectory as the
	// root directory so that it can be passed
	// to the http.FileServer
	fsys, err := fs.Sub(embeddedFiles, "app/build")
	if err != nil {
		panic(err)
	}

	return http.FS(fsys)
}
