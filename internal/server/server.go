package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"path"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/leancodebox/rooster/assets"
	"github.com/leancodebox/rooster/internal/jobmanager"
)

var srv *http.Server
var serverPort int

func ServeRun() *http.Server {
	port := jobmanager.GetHttpConfig().Dashboard.Port
	if port <= 0 {
		return nil
	}

	gin.DisableConsoleColor()
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	srv = &http.Server{
		Handler:        r,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	r.Use(GinCors)
	r.NoRoute(func(c *gin.Context) {
		c.Redirect(http.StatusTemporaryRedirect, "/actor")
	})

	actV3 := r.Group("actor")
	static, err := fs.Sub(assets.GetActorV3Fs(), path.Join("static", "dist"))
	if err == nil {
		actV3.StaticFS("", http.FS(static))
	} else {
		actV3.GET("/*any", func(c *gin.Context) { c.String(http.StatusOK, "dashboard未构建，请先构建前端") })
	}

	api := r.Group("api")

	// System handlers
	api.GET("/home-path", handleHomePath)
	api.GET("/run-info", handleRunInfo)

	// Job handlers
	api.GET("/job-list", handleJobList)
	api.POST("/run-job-resident-task", handleRunJobResidentTask)
	api.POST("/stop-job-resident-task", handleStopJobResidentTask)
	api.POST("/open-close-task", handleOpenCloseTask)
	api.POST("/run-task", handleRunTask)
	api.POST("/save-task", handleSaveTask)
	api.POST("/remove-task", handleRemoveTask)

	// Log handlers
	api.GET("/job-log-list", handleJobLogList)
	api.GET("/job-log", handleJobLog)
	api.GET("/job-log-download", handleJobLogDownload)
	api.GET("/job-log-stream", handleJobLogStream)

	go func() {
		t := time.NewTicker(5 * time.Minute)
		defer t.Stop()
		for {
			<-t.C
			jobmanager.TrimMemLogs(1*time.Hour, 16<<20)
		}
	}()

	var ln net.Listener
	for i := 0; i < 1000; i++ {
		tryPort := port + i
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%v", tryPort))
		if err == nil {
			ln = l
			serverPort = tryPort
			srv.Addr = fmt.Sprintf("127.0.0.1:%v", tryPort)
			slog.Info(fmt.Sprintf("rooster 开启server服务 http://localhost:%v/actor", tryPort))
			break
		}
	}
	if ln == nil {
		slog.Error("无法绑定端口", "start", port)
		serverPort = 0
		return nil
	}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Serve 失败", "err", err.Error())
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

func GetPort() int {
	return serverPort
}
