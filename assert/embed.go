package assert

import (
	"embed"
	"io/fs"
)

//go:embed all:static/**
var distFS embed.FS

func GetActorV3Fs() fs.FS {
	return distFS
}
