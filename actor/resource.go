package actor

import (
	"embed"
)

//go:embed  all:dist/**
var actorFS embed.FS

func GetActorFs() embed.FS {
	return actorFS
}
