package httpapi

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leancodebox/rooster/internal/domain"
	"github.com/leancodebox/rooster/internal/engine"
	"github.com/leancodebox/rooster/internal/runner"
	storesqlite "github.com/leancodebox/rooster/internal/store/sqlite"
)

func TestBackupAndExportAPI(t *testing.T) {
	dir := t.TempDir()
	st, err := storesqlite.Open(filepath.Join(dir, "source.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	eng := engine.New(st, runner.New(runner.NewEnvironmentProvider()), filepath.Join(dir, "logs"))
	task, err := eng.CreateTask(context.Background(), domain.Task{Name: "exported", Kind: domain.TaskKindResident, CommandMode: domain.CommandModeShell, Command: "echo backup", OverlapPolicy: domain.OverlapSkip, RestartPolicy: domain.RestartNever})
	if err != nil {
		t.Fatal(err)
	}
	server := New(eng, nil)
	retention := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(retention, httptest.NewRequest(http.MethodPost, "/api/maintenance/retention", strings.NewReader("{}")))
	var plan engine.RetentionResult
	if err := json.Unmarshal(retention.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if retention.Code != http.StatusOK || !plan.DryRun || plan.Deleted != 0 {
		t.Fatalf("retention should default to preview: status=%d plan=%+v", retention.Code, plan)
	}
	exported := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(exported, httptest.NewRequest(http.MethodGet, "/api/export", nil))
	var payload struct {
		Version int
		Tasks   []domain.Task
	}
	if err := json.Unmarshal(exported.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if exported.Code != http.StatusOK || payload.Version != 1 || len(payload.Tasks) != 1 || payload.Tasks[0].ID != task.ID {
		t.Fatalf("unexpected export: %d %#v", exported.Code, payload)
	}
	backup := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(backup, httptest.NewRequest(http.MethodGet, "/api/backup", nil))
	if backup.Code != http.StatusOK || backup.Header().Get("Content-Disposition") == "" {
		t.Fatalf("backup response: %d", backup.Code)
	}
	path := filepath.Join(dir, "restored.db")
	if err := os.WriteFile(path, backup.Body.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	restored, err := storesqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if _, err := restored.GetTask(context.Background(), task.ID); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "large.log")
	log := strings.Repeat("line\n", 20000)
	if err := os.WriteFile(logPath, []byte(log), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = st.CreateExecution(context.Background(), domain.Execution{ID: "download-test", TaskID: task.ID, Trigger: domain.TriggerManual, Status: domain.ExecutionSucceeded, LogPath: logPath})
	if err != nil {
		t.Fatal(err)
	}
	for _, download := range []bool{false, true} {
		url := "/api/executions/download-test/logs"
		if download {
			url += "?download=1"
		}
		response := httptest.NewRecorder()
		server.http.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, url, nil))
		want := 64 * 1024
		if download {
			want = len(log)
		}
		if response.Code != http.StatusOK || response.Body.Len() != want {
			t.Fatalf("download=%v status=%d bytes=%d want=%d", download, response.Code, response.Body.Len(), want)
		}
	}
	var compressed bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressed)
	if _, err := gzipWriter.Write([]byte(log)); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	archiveName := "large-2026-01-01T00-00-00.000.log.gz"
	if err := os.WriteFile(filepath.Join(dir, archiveName), compressed.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	listing := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(listing, httptest.NewRequest(http.MethodGet, "/api/executions/download-test/logfiles", nil))
	var files struct {
		Archives []logFile `json:"archives"`
	}
	if err := json.Unmarshal(listing.Body.Bytes(), &files); err != nil {
		t.Fatal(err)
	}
	if len(files.Archives) != 1 || files.Archives[0].Name != archiveName {
		t.Fatalf("unexpected archives: %+v", files)
	}
	for _, suffix := range []string{"", "&download=1"} {
		response := httptest.NewRecorder()
		server.http.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/executions/download-test/logs?archive="+archiveName+suffix, nil))
		want := []byte(log[len(log)-(64<<10):])
		if suffix != "" {
			want = compressed.Bytes()
		}
		if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), want) {
			t.Fatalf("archive response: %d bytes=%d", response.Code, response.Body.Len())
		}
	}
	invalid := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(invalid, httptest.NewRequest(http.MethodGet, "/api/executions/download-test/logs?archive=../source.db", nil))
	if invalid.Code != http.StatusNotFound {
		t.Fatalf("path traversal status = %d", invalid.Code)
	}
}
