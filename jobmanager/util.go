package jobmanager

import (
	"time"

	"github.com/google/uuid"
)

var uuidGen = uuid.NewRandom

func generateUUID() string {
	var UUID uuid.UUID
	UUID, err := uuidGen()
	if err != nil {
		return time.Now().Format(time.UnixDate)
	}
	return UUID.String()
}
