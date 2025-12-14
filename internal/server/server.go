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
var shutdownCtx context.Context
var shutdownCancel context.CancelFunc

func timeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func ServeRun() *http.Server {
	port := jobmanager.GetHttpConfig().Dashboard.Port
	if port <= 0 {
		return nil
	}

	shutdownCtx, shutdownCancel = context.WithCancel(context.Background())

	gin.DisableConsoleColor()
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	srv = &http.Server{
		Handler:        r,
		ReadTimeout:    10 * time.Second,
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

	// Log handlers (No timeout)
	api.GET("/job-log-stream", handleJobLogStream)

	// Standard handlers (With 10s timeout)
	stdApi := api.Group("/")
	stdApi.Use(timeoutMiddleware(10 * time.Second))
	{
		// System handlers
		stdApi.GET("/home-path", handleHomePath)
		stdApi.GET("/run-info", handleRunInfo)

		// Job handlers
		stdApi.GET("/job-list", handleJobList)
		stdApi.POST("/run-job-resident-task", handleRunJobResidentTask)
		stdApi.POST("/stop-job-resident-task", handleStopJobResidentTask)
		stdApi.POST("/open-close-task", handleOpenCloseTask)
		stdApi.POST("/run-task", handleRunTask)
		stdApi.POST("/save-task", handleSaveTask)
		stdApi.POST("/remove-task", handleRemoveTask)
	}

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

	// Cancel global context to notify handlers (e.g. log stream) to exit immediately
	if shutdownCancel != nil {
		shutdownCancel()
	}

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
