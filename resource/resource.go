package resource

import (
	_ "embed"
)

//go:embed  jobConfig.example.json
var jobConfigDefault []byte

func GetJobConfigDefault() []byte {
	return jobConfigDefault
}
