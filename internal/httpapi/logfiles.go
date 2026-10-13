package httpapi

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/leancodebox/rooster/internal/store"
)

type logFile struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

func archivesFor(path string) ([]logFile, error) {
	entries, err := os.ReadDir(filepath.Dir(path))
	if errors.Is(err, os.ErrNotExist) {
		return []logFile{}, nil
	}
	if err != nil {
		return nil, err
	}
	stem := strings.TrimSuffix(filepath.Base(path), ".log")
	files := make([]logFile, 0)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, stem+"-") || (!strings.HasSuffix(name, ".log.gz") && !strings.HasSuffix(name, ".log")) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		files = append(files, logFile{Name: name, Size: info.Size()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name > files[j].Name })
	return files, nil
}

func (s *Server) executionLogFiles(w http.ResponseWriter, r *http.Request) {
	execution, err := s.engine.GetExecution(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	files, err := archivesFor(execution.LogPath)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"archives": files})
}

func (s *Server) archiveLog(w http.ResponseWriter, r *http.Request, currentPath, name string) {
	if filepath.Base(name) != name || strings.ContainsAny(name, `/\`) {
		writeError(w, store.ErrNotFound)
		return
	}
	files, err := archivesFor(currentPath)
	if err != nil {
		writeError(w, err)
		return
	}
	found := false
	for _, file := range files {
		if file.Name == name {
			found = true
			break
		}
	}
	if !found {
		writeError(w, store.ErrNotFound)
		return
	}
	file, err := os.Open(filepath.Join(filepath.Dir(currentPath), name))
	if err != nil {
		writeError(w, store.ErrNotFound)
		return
	}
	defer file.Close()
	w.Header().Set("Cache-Control", "no-store")
	if r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
		_, _ = io.Copy(w, file)
		return
	}
	var reader io.Reader = file
	if strings.HasSuffix(name, ".gz") {
		compressed, err := gzip.NewReader(file)
		if err != nil {
			writeError(w, err)
			return
		}
		defer compressed.Close()
		reader = compressed
	}
	var tail logTail
	const maxDecompressed = 32 << 20
	count, err := io.Copy(&tail, io.LimitReader(reader, maxDecompressed+1))
	if err != nil {
		writeError(w, err)
		return
	}
	if count > maxDecompressed {
		writeError(w, fmt.Errorf("archive exceeds 32 MiB decompression limit"))
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(tail.data)
}

type logTail struct{ data []byte }

func (t *logTail) Write(data []byte) (int, error) {
	length := len(data)
	const maxTail = 64 << 10
	if len(data) >= maxTail {
		t.data = append(t.data[:0], data[len(data)-maxTail:]...)
	} else {
		t.data = append(t.data, data...)
		if len(t.data) > maxTail {
			t.data = append(t.data[:0], t.data[len(t.data)-maxTail:]...)
		}
	}
	return length, nil
}
