package jobmanager

import (
	"fmt"
	"testing"
)

func TestJobListManager(t *testing.T) {
	jlm := JobListManager{}
	jlm.append("asda", "adas", "asda")
	d := jlm.getAll()
	d[1] = "123123"
	fmt.Println(jlm.getAll())
}
