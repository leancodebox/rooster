package server

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/leancodebox/rooster/internal/jobmanager"
)

func handleHomePath(c *gin.Context) {
	h := ""
	if v, err := os.UserHomeDir(); err == nil {
		h = v
	}
	c.JSON(http.StatusOK, gin.H{"home": h})
}

func handleRunInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"start":   jobmanager.GetStartTime().Format("2006-01-02 15:04:05"),
		"runTime": formatDuration(jobmanager.GetRunTime()),
	})
}
