package jobmanagerserver

import (
	"context"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/leancodebox/rooster/actor"
	"github.com/leancodebox/rooster/jobmanager"
	"github.com/leancodebox/rooster/jobmanagerserver/serverinfo"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"path"
	"time"
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
		Addr:           fmt.Sprintf(":%v", port),
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
	act := r.Group("actor")
	act.StaticFS("", PFilSystem("./dist", actor.GetActorFs()))
	api := r.Group("api")
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
		var params jobmanager.JobStatus
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
		var params jobmanager.JobStatus
		_ = c.ShouldBind(&params)
		err := jobmanager.RemoveTask(params)
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
