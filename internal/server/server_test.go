package server

import (
	"fmt"
	"testing"
	"time"
)

func TestName(t *testing.T) {
	old := time.Now().Add(-time.Second*86400*101 - time.Second*86400*3 - time.Second*864)
	d := time.Now().Sub(old)

	fmt.Println(formatDuration(d))

	day := int64(d.Hours() / 24)
	hour := int64((d % (time.Hour * 24)).Hours())
	minute := int64((d % time.Hour).Minutes())
	seconds := int64((d % time.Minute).Seconds())
	res := fmt.Sprintf("%02d天%02d时%02d分%02d秒", day, hour, minute, seconds)
	fmt.Println(res)

}
