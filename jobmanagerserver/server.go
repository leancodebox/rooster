package jobmanagerserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/leancodebox/rooster/assert"
	"github.com/leancodebox/rooster/jobmanager"
	"github.com/leancodebox/rooster/jobmanagerserver/serverinfo"
)

var srv *http.Server

func ServeRun() *http.Server {
	port := jobmanager.GetHttpConfig().Dashboard.Port
	if port <= 0 {
		return nil
	}
	slog.Info(fmt.Sprintf("rooster 开启server服务 http://localhost:%v/actor", port))
	//r := gin.Default()

	gin.DisableConsoleColor()
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	srv = &http.Server{
		Addr:           fmt.Sprintf("127.0.0.1:%v", port),
		Handler:        r,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	r.Use(GinCors)
	r.Use(IpLimit)
	r.NoRoute(func(c *gin.Context) {
		c.Redirect(http.StatusTemporaryRedirect, "/actor")
	})
	actV3 := r.Group("actor")
	static, _ := fs.Sub(assert.GetActorV3Fs(), path.Join("static", "dist"))
	actV3.StaticFS("", http.FS(static))
	api := r.Group("api")
	go func() {
		t := time.NewTicker(5 * time.Minute)
		defer t.Stop()
		for {
			<-t.C
			jobmanager.TrimMemLogs(1*time.Hour, 16<<20)
		}
	}()
	api.GET("/home-path", func(c *gin.Context) {
		h := ""
		if v, err := os.UserHomeDir(); err == nil {
			h = v
		}
		c.JSON(http.StatusOK, gin.H{"home": h})
	})
	api.GET("/job-list", func(c *gin.Context) {
		all := jobmanager.JobList()
		c.JSON(http.StatusOK, gin.H{
			"message": all,
		})
	})
	type JobUpdateReq struct {
		JobId string `json:"jobId"`
	}
	api.POST("/run-job-resident-task", func(c *gin.Context) {
		var params JobUpdateReq
		_ = c.ShouldBind(&params)
		err := jobmanager.JobRunResidentTask(params.JobId)
		msg := "success"
		if err != nil {
			msg = err.Error()
		}
		c.JSON(http.StatusOK, gin.H{
			"message": msg,
		})
	})
	api.POST("/stop-job-resident-task", func(c *gin.Context) {
		var params JobUpdateReq
		_ = c.ShouldBind(&params)
		err := jobmanager.JobStopResidentTask(params.JobId)
		msg := "success"
		if err != nil {
			msg = err.Error()
		}
		c.JSON(http.StatusOK, gin.H{
			"message": msg,
		})
	})

	type RunOpenCloseTask struct {
		UUID string `json:"uuid"`
		Run  bool   `json:"run"`
	}

	api.POST("/open-close-task", func(c *gin.Context) {
		var params RunOpenCloseTask
		_ = c.ShouldBind(&params)
		err := jobmanager.OpenCloseTask(params.UUID, params.Run)
		msg := "success"
		if err != nil {
			msg = err.Error()
		}
		c.JSON(http.StatusOK, gin.H{
			"message": msg,
		})
	})

	type TaskActionReq struct {
		TaskId string `json:"taskId"`
	}
	api.POST("/run-task", func(c *gin.Context) {
		var params TaskActionReq
		_ = c.ShouldBind(&params)
		err := jobmanager.RunTask(params.TaskId)
		msg := "success"
		if err != nil {
			msg = err.Error()
		}
		c.JSON(http.StatusOK, gin.H{
			"message": msg,
		})
	})

	api.POST("/save-task", func(c *gin.Context) {
		var params jobmanager.JobStatusShow
		_ = c.ShouldBind(&params)
		err := jobmanager.SaveTask(params)
		msg := "success"
		if err != nil {
			msg = err.Error()
		}
		c.JSON(http.StatusOK, gin.H{
			"message": msg,
		})
	})

	api.POST("/remove-task", func(c *gin.Context) {
		var req struct {
			UUID  string `json:"uuid"`
			JobId string `json:"jobId"`
		}
		_ = c.ShouldBind(&req)
		id := req.UUID
		if id == "" {
			id = req.JobId
		}
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"message": "uuid/jobId缺失"})
			return
		}
		err := jobmanager.RemoveTask(jobmanager.JobStatusShow{UUID: id})
		msg := "success"
		if err != nil {
			msg = err.Error()
		}
		c.JSON(http.StatusOK, gin.H{
			"message": msg,
		})
	})

	api.GET("/run-info", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "success",
			"start":   jobmanager.GetStartTime().Format("2006-01-02 15:04:05"),
			"runTime": formatDuration(jobmanager.GetRunTime()),
		})
	})

	// logs: list
	api.GET("/job-log-list", func(c *gin.Context) {
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
	})

	// logs: tail
	api.GET("/job-log", func(c *gin.Context) {
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
	})

	// logs: download
	api.GET("/job-log-download", func(c *gin.Context) {
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
	})

	// logs: stream (SSE)
	api.GET("/job-log-stream", func(c *gin.Context) {
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
			}
			time.Sleep(500 * time.Millisecond)
		}
	})

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	return srv
}
func ServeStop() {
	if srv == nil {
		return
	}
	slog.Info("Shutdown Server ...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Info("Server Shutdown:", "err", err.Error())
	}
	jobmanager.StopAll()
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

type fsFunc func(name string) (fs.File, error)

func (f fsFunc) Open(name string) (fs.File, error) {
	return f(name)
}

func upFsHandle(pPath string, fSys fs.FS) fsFunc {
	return func(name string) (fs.File, error) {
		assetPath := path.Join(pPath, name)
		// If we can't find the asset, fs can handle the error
		file, err := fSys.Open(assetPath)
		if err != nil {
			slog.Error(err.Error())
			return nil, err
		}
		return file, err
	}
}

func PFilSystem(pPath string, fSys fs.FS) http.FileSystem {
	return http.FS(upFsHandle(pPath, fSys))
}

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

func IpLimit(c *gin.Context) {
	clientIP := c.ClientIP()
	ip, _ := serverinfo.GetLocalIp()
	if len(ip) != 0 && clientIP != ip && clientIP != "::1" && clientIP != "127.0.0.1" {
		slog.Error("ipLimit", "clientIp", clientIP, "localIp", ip)
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}
	c.Next()
}

func formatDuration(d time.Duration) string {
	day := int64(d.Hours() / 24)
	hour := int64((d % (time.Hour * 24)).Hours())
	minute := int64((d % time.Hour).Minutes())
	seconds := int64((d % time.Minute).Seconds())
	return fmt.Sprintf("%02d天%02d时%02d分%02d秒", day, hour, minute, seconds)
}

// moved route handlers inside ServeRun earlier
