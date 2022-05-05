package web

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/internal/core"
)

type WorkflowInstanceRef struct {
	Instance    *core.WorkflowInstance `json:"instance,omitempty"`
	CreatedAt   time.Time              `json:"created_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	State       backend.WorkflowState  `json:"state,omitempty"`
}

type Event struct {
	ID              string      `json:"id,omitempty"`
	SequenceID      int64       `json:"sequence_id,omitempty"`
	Type            string      `json:"type,omitempty"`
	Timestamp       time.Time   `json:"timestamp,omitempty"`
	ScheduleEventID int64       `json:"schedule_event_id,omitempty"`
	Attributes      interface{} `json:"attributes,omitempty"`
	VisibleAt       *time.Time  `json:"visible_at,omitempty"`
}

type WorkflowInstanceInfo struct {
	*WorkflowInstanceRef

	History []*Event `json:"history,omitempty"`
}

type WebBackend interface {
	backend.Backend

	GetWorkflowInstance(ctx context.Context, instanceID string) (*WorkflowInstanceRef, error)
	GetWorkflowInstances(ctx context.Context, afterInstanceID string, count int) ([]*WorkflowInstanceRef, error)
}

//go:embed app/build
var embeddedFiles embed.FS

func NewMux(backend WebBackend) *http.ServeMux {
	mux := http.NewServeMux()

	// API
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		// Only support GET requests
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		relativeURL := strings.TrimPrefix(r.URL.Path, "/api/")

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

			instances, err := backend.GetWorkflowInstances(r.Context(), query.Get("after"), count)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.Header().Add("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(instances); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			return
		}

		segments := strings.Split(relativeURL, "/")

		// /api/{instanceID}
		if len(segments) == 1 {
			instanceID := segments[0]

			instance, err := backend.GetWorkflowInstance(r.Context(), instanceID)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if instance == nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			history, err := backend.GetWorkflowInstanceHistory(r.Context(), instance.Instance, nil)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
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
				WorkflowInstanceRef: instance,
				History:             newHistory,
			}

			w.Header().Add("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(result); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
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
