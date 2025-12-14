package server

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/leancodebox/rooster/internal/jobmanager"
)

func GinCors(context *gin.Context) {
	method := context.Request.Method
	context.Header("Access-Control-Allow-Origin", "*")
	context.Header("Access-Control-Allow-Headers", "Content-Type,AccessToken,X-CSRF-Token, Authorization, Token, New-Token")
	context.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, DELETE, PATCH, PUT")
	context.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Content-Type,New-Token")
	context.Header("Access-Control-Allow-Credentials", "true")
	if method == "OPTIONS" {
		context.AbortWithStatus(http.StatusNoContent)
	}
	context.Next()
}

func formatDuration(d time.Duration) string {
	day := int64(d.Hours() / 24)
	hour := int64((d % (time.Hour * 24)).Hours())
	minute := int64((d % time.Hour).Minutes())
	seconds := int64((d % time.Minute).Seconds())
	return fmt.Sprintf("%02d天%02d时%02d分%02d秒", day, hour, minute, seconds)
}

func getJobStatusById(id string) (jobmanager.JobStatusShow, bool) {
	for _, j := range jobmanager.JobList() {
		if j.UUID == id {
			return j, true
		}
	}
	return jobmanager.JobStatusShow{}, false
}

func getJobLogPath(j jobmanager.JobStatusShow) (string, bool) {
	if j.Options.OutputType != 2 || j.Options.OutputPath == "" {
		return "", false
	}
	p := filepath.Join(j.Options.OutputPath, j.JobName+"_log.txt")
	// safety: ensure p starts with OutputPath
	if !strings.HasPrefix(p, j.Options.OutputPath) {
		return "", false
	}
	return p, true
}

func readTail(path string, lines, bytes, maxBytes int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if bytes > 0 {
		if bytes > maxBytes {
			bytes = maxBytes
		}
		off := st.Size() - int64(bytes)
		if off < 0 {
			off = 0
		}
		_, _ = f.Seek(off, io.SeekStart)
		buf := make([]byte, st.Size()-off)
		n, _ := io.ReadFull(f, buf)
		return buf[:n], nil
	}
	// read lines tail
	// naive implementation: read last maxBytes then split
	sz := st.Size()
	read := int64(maxBytes)
	if read > sz {
		read = sz
	}
	_, _ = f.Seek(sz-read, io.SeekStart)
	buf := make([]byte, read)
	n, _ := io.ReadFull(f, buf)
	parts := strings.Split(string(buf[:n]), "\n")
	if len(parts) <= lines {
		return buf[:n], nil
	}
	tail := strings.Join(parts[len(parts)-lines-1:], "\n")
	return []byte(tail), nil
}
