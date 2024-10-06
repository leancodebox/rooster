package resource

import (
	_ "embed"
	"runtime"
)

//go:embed  jobConfig.example.json
var jobConfigDefault []byte

//go:embed  jobConfig.example.win.json
var jobConfigDefault4win []byte

func GetJobConfigDefault() []byte {
	sysType := runtime.GOOS

	if sysType == `windows` {
		return jobConfigDefault4win
	} else {
		return jobConfigDefault
	}
}
