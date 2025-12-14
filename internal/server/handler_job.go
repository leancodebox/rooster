package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/leancodebox/rooster/internal/jobmanager"
)

type JobUpdateReq struct {
	JobId string `json:"jobId"`
}

type RunOpenCloseTask struct {
	UUID string `json:"uuid"`
	Run  bool   `json:"run"`
}

type TaskActionReq struct {
	TaskId string `json:"taskId"`
}

func handleJobList(c *gin.Context) {
	all := jobmanager.JobList()
	c.JSON(http.StatusOK, gin.H{
		"message": all,
	})
}

func handleRunJobResidentTask(c *gin.Context) {
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
}

func handleStopJobResidentTask(c *gin.Context) {
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
}

func handleOpenCloseTask(c *gin.Context) {
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
}

func handleRunTask(c *gin.Context) {
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
}

func handleSaveTask(c *gin.Context) {
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
}

func handleRemoveTask(c *gin.Context) {
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
}
