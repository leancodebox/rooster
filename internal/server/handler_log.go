package server

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/leancodebox/rooster/internal/jobmanager"
)

func handleJobLogList(c *gin.Context) {
	var out []gin.H
	for _, j := range jobmanager.JobList() {
		hasLog := false
		lp := ""
		size := int64(0)
		mt := ""
		if j.Options.OutputType == 2 && j.Options.OutputPath != "" {
			lp = filepath.Join(j.Options.OutputPath, j.JobName+"_log.txt")
			if st, err := os.Stat(lp); err == nil && !st.IsDir() {
				hasLog = true
				size = st.Size()
				mt = st.ModTime().Format("2006-01-02 15:04:05")
			}
		}
		if !hasLog {
			if sz, lt, ok := jobmanager.GetMemLogStat(j.UUID); ok {
				hasLog = true
				size = sz
				mt = lt.Format("2006-01-02 15:04:05")
			}
		}
		out = append(out, gin.H{
			"uuid":    j.UUID,
			"jobName": j.JobName,
			"hasLog":  hasLog,
			"logPath": lp,
			"size":    size,
			"modTime": mt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"message": out})
}

func handleJobLog(c *gin.Context) {
	jobId := c.Query("jobId")
	lines := 200
	bytes := 0
	if v := c.Query("lines"); v != "" {
		fmt.Sscanf(v, "%d", &lines)
	}
	if v := c.Query("bytes"); v != "" {
		fmt.Sscanf(v, "%d", &bytes)
	}
	j, ok := getJobStatusById(jobId)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"message": "jobId不存在"})
		return
	}
	lp, ok := getJobLogPath(j)
	maxBytes := 2 * 1024 * 1024
	if bytes <= 0 {
		bytes = 0
	}
	if ok {
		data, err := readTail(lp, lines, bytes, maxBytes)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"content": string(data)})
		return
	}
	if data, ok := jobmanager.ReadMemTail(j.UUID, lines, bytes, maxBytes); ok {
		c.JSON(http.StatusOK, gin.H{"content": string(data)})
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"message": "未开启文件日志或日志为空"})
}

func handleJobLogDownload(c *gin.Context) {
	jobId := c.Query("jobId")
	j, ok := getJobStatusById(jobId)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"message": "jobId不存在"})
		return
	}
	lp, ok := getJobLogPath(j)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"message": "未开启文件日志或路径无效"})
		return
	}
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s_log.txt\"", j.JobName))
	c.File(lp)
}

func handleJobLogStream(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	jobId := c.Query("jobId")
	j, ok := getJobStatusById(jobId)
	if !ok {
		c.String(http.StatusNotFound, "")
		return
	}
	lp, ok := getJobLogPath(j)
	if !ok {
		c.String(http.StatusBadRequest, "")
		return
	}
	f, err := os.Open(lp)
	if err != nil {
		c.String(http.StatusInternalServerError, "")
		return
	}
	defer f.Close()
	pos := int64(0)
	if st, err := f.Stat(); err == nil {
		pos = st.Size()
	}
	lastEmit := time.Now()
	for {
		if c.Request.Context().Err() != nil {
			break
		}
		st, err := os.Stat(lp)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		size := st.Size()
		if size < pos {
			pos = 0
		}
		if size > pos {
			_, _ = f.Seek(pos, io.SeekStart)
			buf := make([]byte, size-pos)
			n, _ := io.ReadFull(f, buf)
			pos += int64(n)
			c.Writer.Write([]byte("data: "))
			c.Writer.Write(buf[:n])
			c.Writer.Write([]byte("\n\n"))
			c.Writer.Flush()
			lastEmit = time.Now()
		} else {
			if time.Since(lastEmit) >= 10*time.Second {
				c.Writer.Write([]byte(": keep-alive\n\n"))
				c.Writer.Flush()
				lastEmit = time.Now()
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
}
