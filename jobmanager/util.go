package jobmanager

import (
	"github.com/google/uuid"
	"time"
)

func generateUUID() string {
	var UUID uuid.UUID
	UUID, err := uuid.NewRandom()
	if err != nil {
		return time.Now().Format(time.UnixDate)
	}
	return UUID.String()
}
