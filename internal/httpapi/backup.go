package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/leancodebox/rooster/internal/domain"
)

func (s *Server) exportTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := s.engine.ExportTasks(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Disposition", `attachment; filename="rooster-tasks.json"`)
	writeJSON(w, http.StatusOK, struct {
		Version    int           `json:"version"`
		ExportedAt time.Time     `json:"exportedAt"`
		Tasks      []domain.Task `json:"tasks"`
	}{Version: 1, ExportedAt: time.Now().UTC(), Tasks: tasks})
}

func (s *Server) backup(w http.ResponseWriter, r *http.Request) {
	// Delay headers until a snapshot exists, so database errors remain JSON errors.
	writer := &backupWriter{ResponseWriter: w}
	if err := s.engine.WriteBackup(r.Context(), writer); err != nil {
		if !writer.started {
			writeError(w, err)
		} else {
			slog.Error("stream database backup", "error", err)
		}
	}
}

type backupWriter struct {
	http.ResponseWriter
	started bool
}

func (w *backupWriter) Write(data []byte) (int, error) {
	if !w.started {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Disposition", `attachment; filename="rooster.db"`)
		w.started = true
	}
	return w.ResponseWriter.Write(data)
}
