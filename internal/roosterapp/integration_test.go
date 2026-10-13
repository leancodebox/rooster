//go:build !windows

package roosterapp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/leancodebox/rooster/internal/domain"
	"github.com/leancodebox/rooster/internal/engine"
	"github.com/leancodebox/rooster/internal/httpapi"
	storesqlite "github.com/leancodebox/rooster/internal/store/sqlite"
)

func TestIsolatedApplicationAcceptance(t *testing.T) {
	t.Setenv("ROOSTER_EXECUTION_TIMEOUT", "2s")
	t.Setenv("ROOSTER_MAX_CONCURRENT", "2")
	t.Setenv("ROOSTER_MAX_PER_TASK", "1")
	root := t.TempDir()
	dir := filepath.Join(root, "original")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	// Seed only our temporary instance with old terminal history for cleanup.
	st, err := storesqlite.Open(filepath.Join(dir, "rooster.db"))
	if err != nil {
		t.Fatal(err)
	}
	fixtureID := uuid.NewString()
	_, err = st.CreateTask(context.Background(), domain.Task{ID: fixtureID, Name: "old-history", Kind: domain.TaskKindResident, CommandMode: domain.CommandModeShell, Command: "echo fixture", OverlapPolicy: domain.OverlapSkip, RestartPolicy: domain.RestartNever})
	if err != nil {
		st.Close()
		t.Fatal(err)
	}
	oldID := uuid.NewString()
	oldPath := filepath.Join(dir, "logs", fixtureID, oldID+".log")
	if err := os.MkdirAll(filepath.Dir(oldPath), 0o700); err != nil {
		st.Close()
		t.Fatal(err)
	}
	if err := os.WriteFile(oldPath, []byte("old output\n"), 0o600); err != nil {
		st.Close()
		t.Fatal(err)
	}
	for index, at := range []time.Time{time.Now().UTC().AddDate(0, 0, -60), time.Now().UTC()} {
		id := oldID
		if index == 1 {
			id = uuid.NewString()
		}
		_, err := st.CreateExecution(context.Background(), domain.Execution{ID: id, TaskID: fixtureID, Trigger: domain.TriggerManual, Status: domain.ExecutionSucceeded, CreatedAt: at, FinishedAt: &at, LogPath: filepath.Join(dir, "logs", fixtureID, id+".log")})
		if err != nil {
			st.Close()
			t.Fatal(err)
		}
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	app, err := Start(context.Background(), dir, "127.0.0.1:0", httpapi.WebAssets())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close(context.Background())
	client := &http.Client{Timeout: 5 * time.Second}
	request := func(base, method, path string, body any, want int, result any) []byte {
		t.Helper()
		var encoded []byte
		if body != nil {
			var err error
			encoded, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
		}
		req, err := http.NewRequest(method, base+path, bytes.NewReader(encoded))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		data, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != want {
			t.Fatalf("%s %s status=%d want=%d body=%s", method, path, response.StatusCode, want, data)
		}
		if result != nil {
			if err := json.Unmarshal(data, result); err != nil {
				t.Fatal(err)
			}
		}
		return data
	}
	await := func(check func() bool) {
		t.Helper()
		deadline := time.Now().Add(8 * time.Second)
		for time.Now().Before(deadline) {
			if check() {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatal("acceptance condition did not become true")
	}
	views := func(base string) []engine.TaskView {
		var result struct {
			Tasks []engine.TaskView `json:"tasks"`
		}
		request(base, http.MethodGet, "/api/tasks", nil, http.StatusOK, &result)
		return result.Tasks
	}
	request(app.URL(), http.MethodGet, "/api/health", nil, http.StatusOK, nil)
	page := request(app.URL(), http.MethodGet, "/", nil, http.StatusOK, nil)
	if !bytes.Contains(page, []byte("Rooster")) {
		t.Fatal("embedded page missing")
	}
	resident := domain.Task{Name: "acceptance-resident", Kind: domain.TaskKindResident, Enabled: true, CommandMode: domain.CommandModeExec, Command: "/bin/sh", Arguments: []string{"-c", "printf 'ready\\n'; sleep 30"}, OverlapPolicy: domain.OverlapSkip, RestartPolicy: domain.RestartAlways}
	request(app.URL(), http.MethodPost, "/api/tasks", resident, http.StatusCreated, &resident)
	var runID string
	await(func() bool {
		for _, view := range views(app.URL()) {
			if view.ID == resident.ID && view.Runtime.ExecutionID != "" {
				runID = view.Runtime.ExecutionID
				return true
			}
		}
		return false
	})
	await(func() bool {
		response, err := client.Get(app.URL() + "/api/executions/" + runID + "/logs")
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		data, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return response.StatusCode == http.StatusOK && bytes.Contains(data, []byte("ready"))
	})
	request(app.URL(), http.MethodPost, "/api/executions/"+runID+"/stop", nil, http.StatusAccepted, nil)
	await(func() bool {
		for _, view := range views(app.URL()) {
			if view.ID == resident.ID {
				return !view.Enabled && view.Runtime.State == "idle"
			}
		}
		return false
	})
	log := request(app.URL(), http.MethodGet, "/api/executions/"+runID+"/logs?download=1", nil, http.StatusOK, nil)
	if !bytes.Contains(log, []byte("ready")) {
		t.Fatal("resident output missing")
	}
	missing := domain.Task{Name: "acceptance-missing", Kind: domain.TaskKindResident, Enabled: true, CommandMode: domain.CommandModeExec, Command: filepath.Join(root, "does-not-exist"), OverlapPolicy: domain.OverlapSkip, RestartPolicy: domain.RestartOnFailure, MaxRetries: 2}
	request(app.URL(), http.MethodPost, "/api/tasks", missing, http.StatusCreated, &missing)
	await(func() bool {
		for _, view := range views(app.URL()) {
			if view.ID == missing.ID {
				return !view.Enabled && view.Runtime.State == "failed" && view.Runtime.RetryCount == 2
			}
		}
		return false
	})
	timed := domain.Task{Name: "acceptance-timeout", Kind: domain.TaskKindScheduled, CommandMode: domain.CommandModeShell, Command: "sleep 30", Schedule: "0 * * * *", OverlapPolicy: domain.OverlapParallel, RestartPolicy: domain.RestartNever}
	request(app.URL(), http.MethodPost, "/api/tasks", timed, http.StatusCreated, &timed)
	request(app.URL(), http.MethodPost, "/api/tasks/"+timed.ID+"/executions", nil, http.StatusAccepted, nil)
	request(app.URL(), http.MethodPost, "/api/tasks/"+timed.ID+"/executions", nil, http.StatusConflict, nil)
	await(func() bool {
		var result struct {
			Executions []domain.Execution `json:"executions"`
		}
		request(app.URL(), http.MethodGet, "/api/tasks/"+timed.ID+"/executions", nil, http.StatusOK, &result)
		return len(result.Executions) == 1 && result.Executions[0].Status == domain.ExecutionFailed && result.Executions[0].Error == context.DeadlineExceeded.Error()
	})
	request(app.URL(), http.MethodPost, "/api/tasks/"+timed.ID+"/enable", nil, http.StatusOK, nil)
	snapshot := request(app.URL(), http.MethodGet, "/api/backup", nil, http.StatusOK, nil)
	backupPath := filepath.Join(root, "downloaded.db")
	if err := os.WriteFile(backupPath, snapshot, 0o600); err != nil {
		t.Fatal(err)
	}
	restoredDir := filepath.Join(root, "restored")
	if err := RestoreSnapshot(context.Background(), backupPath, restoredDir); err != nil {
		t.Fatal(err)
	}
	restored, err := Start(context.Background(), restoredDir, "127.0.0.1:0", httpapi.WebAssets())
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close(context.Background())
	restoredViews := views(restored.URL())
	if len(restoredViews) != 4 {
		t.Fatalf("restored task count=%d", len(restoredViews))
	}
	for _, view := range restoredViews {
		if view.Enabled || view.Runtime.ExecutionID != "" {
			t.Fatalf("restored task unexpectedly active: %+v", view)
		}
	}
	var plan engine.RetentionResult
	request(app.URL(), http.MethodPost, "/api/maintenance/retention", map[string]any{"days": 30, "keepPerTask": 1}, http.StatusOK, &plan)
	if !plan.DryRun || plan.Candidates != 1 || plan.Deleted != 0 {
		t.Fatalf("unexpected cleanup preview: %+v", plan)
	}
	if _, err := os.Stat(oldPath); err != nil {
		t.Fatal("preview removed old log")
	}
	request(app.URL(), http.MethodPost, "/api/maintenance/retention", map[string]any{"days": 30, "keepPerTask": 1, "dryRun": false}, http.StatusOK, &plan)
	if plan.Deleted != 1 || len(plan.Errors) != 0 {
		t.Fatalf("unexpected cleanup result: %+v", plan)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("old log not removed: %v", err)
	}
	request(restored.URL(), http.MethodGet, "/api/executions/"+oldID+"/logs", nil, http.StatusNotFound, nil)
	request(app.URL(), http.MethodGet, "/api/export", nil, http.StatusOK, nil)
}
