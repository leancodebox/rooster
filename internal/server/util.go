package server

import (
	"fmt"
	"net/http"
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
	if j.RealLogPath != "" {
		return j.RealLogPath, true
	}
	return "", false
}
