package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/leancodebox/rooster/internal/domain"
	"github.com/leancodebox/rooster/internal/engine"
	"github.com/leancodebox/rooster/internal/store"
)

type Server struct {
	engine *engine.Engine
	http   *http.Server
	assets fs.FS
}

func New(taskEngine *engine.Engine, assets fs.FS) *Server {
	s := &Server{engine: taskEngine, assets: assets}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/tasks", s.listTasks)
	mux.HandleFunc("POST /api/tasks", s.createTask)
	mux.HandleFunc("PUT /api/tasks/{id}", s.updateTask)
	mux.HandleFunc("DELETE /api/tasks/{id}", s.deleteTask)
	mux.HandleFunc("POST /api/tasks/{id}/enable", s.enableTask)
	mux.HandleFunc("POST /api/tasks/{id}/disable", s.disableTask)
	mux.HandleFunc("POST /api/tasks/{id}/executions", s.runTask)
	mux.HandleFunc("GET /api/tasks/{id}/executions", s.listExecutions)
	mux.HandleFunc("POST /api/executions/{id}/stop", s.stopExecution)
	mux.HandleFunc("GET /api/executions/{id}/logs", s.executionLogs)
	mux.Handle("/", s.webHandler())
	s.http = &http.Server{Handler: requestLog(mux), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 0, IdleTimeout: 60 * time.Second}
	return s
}

func (s *Server) Serve(listener net.Listener) error  { return s.http.Serve(listener) }
func (s *Server) Shutdown(ctx context.Context) error { return s.http.Shutdown(ctx) }

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
func (s *Server) listTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := s.engine.ListTasks(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": tasks})
}
func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	var task domain.Task
	if err := decodeJSON(r, &task); err != nil {
		writeError(w, err)
		return
	}
	created, err := s.engine.CreateTask(r.Context(), task)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}
func (s *Server) updateTask(w http.ResponseWriter, r *http.Request) {
	var request struct {
		domain.Task
		Runtime  json.RawMessage `json:"runtime"`
		NextRuns json.RawMessage `json:"nextRuns"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, err)
		return
	}
	task := request.Task
	task.ID = r.PathValue("id")
	updated, err := s.engine.UpdateTask(r.Context(), task)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}
func (s *Server) deleteTask(w http.ResponseWriter, r *http.Request) {
	if err := s.engine.DeleteTask(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) enableTask(w http.ResponseWriter, r *http.Request)  { s.setEnabled(w, r, true) }
func (s *Server) disableTask(w http.ResponseWriter, r *http.Request) { s.setEnabled(w, r, false) }
func (s *Server) setEnabled(w http.ResponseWriter, r *http.Request, enabled bool) {
	if err := s.engine.SetEnabled(r.Context(), r.PathValue("id"), enabled); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"enabled": enabled})
}
func (s *Server) runTask(w http.ResponseWriter, r *http.Request) {
	execution, err := s.engine.Run(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, execution)
}
func (s *Server) stopExecution(w http.ResponseWriter, r *http.Request) {
	if err := s.engine.StopExecution(r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "stopping"})
}
func (s *Server) listExecutions(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := s.engine.ListExecutions(r.Context(), r.PathValue("id"), limit)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"executions": items})
}

func (s *Server) executionLogs(w http.ResponseWriter, r *http.Request) {
	execution, err := s.engine.GetExecution(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	f, err := os.Open(execution.LogPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, store.ErrNotFound)
		} else {
			writeError(w, err)
		}
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if info, statErr := f.Stat(); statErr == nil && info.Size() > 64*1024 {
		_, _ = f.Seek(info.Size()-64*1024, io.SeekStart)
	}
	if _, err := io.Copy(w, f); err != nil {
		slog.Debug("write log response", "error", err)
	}
}

func (s *Server) webHandler() http.Handler {
	if s.assets == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "web assets are not built", http.StatusServiceUnavailable)
		})
	}
	files := http.FileServer(http.FS(s.assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(s.assets, path); err != nil {
			clone := r.Clone(r.Context())
			clone.URL.Path = "/index.html"
			files.ServeHTTP(w, clone)
			return
		}
		files.ServeHTTP(w, r)
	})
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid request: %w", err)
	}
	return nil
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, err error) {
	status := http.StatusUnprocessableEntity
	if errors.Is(err, store.ErrNotFound) {
		status = http.StatusNotFound
	} else if errors.Is(err, engine.ErrAlreadyRunning) {
		status = http.StatusConflict
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Debug("http request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start))
	})
}
