package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/leancodebox/rooster/internal/domain"
	"github.com/leancodebox/rooster/internal/engine"
	"github.com/leancodebox/rooster/internal/runner"
	storesqlite "github.com/leancodebox/rooster/internal/store/sqlite"
)

func TestTaskAPI(t *testing.T) {
	dir := t.TempDir()
	st, err := storesqlite.Open(filepath.Join(dir, "rooster.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	eng := engine.New(st, runner.New(runner.NewEnvironmentProvider()), filepath.Join(dir, "logs"))
	if err := eng.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = eng.Close(ctx)
	}()
	api := New(eng, nil)
	server := httptest.NewServer(api.http.Handler)
	defer server.Close()
	task := domain.Task{Name: "worker", Link: "https://example.com", Kind: domain.TaskKindResident, CommandMode: domain.CommandModeShell, Command: "echo ready", OverlapPolicy: domain.OverlapSkip, RestartPolicy: domain.RestartNever}
	body, _ := json.Marshal(task)
	response, err := http.Post(server.URL+"/api/tasks", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d", response.StatusCode)
	}
	response, err = http.Get(server.URL + "/api/tasks")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d", response.StatusCode)
	}
	var result struct {
		Tasks []engine.TaskView `json:"tasks"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Tasks) != 1 || result.Tasks[0].Name != "worker" || result.Tasks[0].Link != task.Link {
		t.Fatalf("unexpected tasks: %#v", result.Tasks)
	}
}

func TestWebAssetsContainIndex(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()
	New(nil, WebAssets()).webHandler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestEmptyTaskListIsJSONArray(t *testing.T) {
	dir := t.TempDir()
	st, err := storesqlite.Open(filepath.Join(dir, "rooster.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	eng := engine.New(st, runner.New(runner.NewEnvironmentProvider()), filepath.Join(dir, "logs"))
	request := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	recorder := httptest.NewRecorder()
	New(eng, nil).http.Handler.ServeHTTP(recorder, request)
	var result struct {
		Tasks json.RawMessage `json:"tasks"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if string(result.Tasks) != "[]" {
		t.Fatalf("tasks JSON = %s, want []", result.Tasks)
	}
}

func TestEmptyExecutionListIsJSONArray(t *testing.T) {
	dir := t.TempDir()
	st, err := storesqlite.Open(filepath.Join(dir, "rooster.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	eng := engine.New(st, runner.New(runner.NewEnvironmentProvider()), filepath.Join(dir, "logs"))
	task, err := eng.CreateTask(context.Background(), domain.Task{Name: "idle", Kind: domain.TaskKindResident, CommandMode: domain.CommandModeShell, Command: "echo ready", OverlapPolicy: domain.OverlapSkip, RestartPolicy: domain.RestartNever})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/tasks/"+task.ID+"/executions", nil)
	request.SetPathValue("id", task.ID)
	recorder := httptest.NewRecorder()
	New(eng, nil).http.Handler.ServeHTTP(recorder, request)
	var result struct {
		Executions json.RawMessage `json:"executions"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if string(result.Executions) != "[]" {
		t.Fatalf("executions JSON = %s, want []", result.Executions)
	}
}
