package diag

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"github.com/cschleiden/go-workflows/core"
)

//go:embed app/build
var embeddedFiles embed.FS

// NewServeMux returns an *http.ServeMux that serves the diagnostics web app at / and the diagnostics API at /api which is
// used by the web app.
func NewServeMux(backend Backend) *http.ServeMux {
	mux := http.NewServeMux()

	// API
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		// Only support GET requests
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		relativeURL := strings.TrimPrefix(r.URL.Path, "/api/")

		// /api/stats
		if relativeURL == "stats" {
			stats, err := backend.GetStats(r.Context())
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(err.Error()))
				return
			}

			w.Header().Add("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(stats); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(err.Error()))
				return
			}

			return
		}

		// /api/
		if relativeURL == "" {
			// Index
			query := r.URL.Query()

			count := 25
			countStr := query.Get("count")
			if countStr != "" {
				var err error
				count, err = strconv.Atoi(countStr)
				if err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
			}
			after := query.Get("after")
			segments := strings.Split(after, ":")
			var afterInstanceID, afterExecutionID string
			if len(segments) == 2 {
				afterInstanceID = segments[0]
				afterExecutionID = segments[1]
			}

			instances, err := backend.GetWorkflowInstances(r.Context(), afterInstanceID, afterExecutionID, count)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(err.Error()))
				return
			}

			w.Header().Add("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(instances); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(err.Error()))
				return
			}

			return
		}

		segments := strings.Split(relativeURL, "/")

		// /api/{instanceID}/{executionID}
		if len(segments) == 2 {
			instanceID := segments[0]
			executionID := segments[1]
			instance := core.NewWorkflowInstance(instanceID, executionID)

			instanceRef, err := backend.GetWorkflowInstance(r.Context(), instance)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			if instanceRef == nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			history, err := backend.GetWorkflowInstanceHistory(r.Context(), instanceRef.Instance, nil)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(err.Error()))
				return
			}

			newHistory := make([]*Event, 0)
			for _, event := range history {
				newHistory = append(newHistory, &Event{
					ID:              event.ID,
					SequenceID:      event.SequenceID,
					Type:            event.Type.String(),
					Timestamp:       event.Timestamp,
					ScheduleEventID: event.ScheduleEventID,
					Attributes:      event.Attributes,
					VisibleAt:       event.VisibleAt,
				})
			}

			result := &WorkflowInstanceInfo{
				WorkflowInstanceRef: instanceRef,
				History:             newHistory,
			}

			w.Header().Add("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(result); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(err.Error()))
				return
			}

			return
		}

		// /api/{instanceID}/{executionID}/tree
		if len(segments) == 3 {
			instanceID := segments[0]
			executionID := segments[1]
			op := segments[2]
			if op != "tree" {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			tree, err := backend.GetWorkflowTree(r.Context(), core.NewWorkflowInstance(instanceID, executionID))
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			if tree == nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			w.Header().Add("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(tree); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(err.Error()))
				return
			}

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
