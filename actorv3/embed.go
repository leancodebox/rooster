package actorv3

import (
    "embed"
    "io/fs"
)

//go:embed dist/* dist/**/*
var distFS embed.FS

func GetActorV3Fs() fs.FS {
    return distFS
}
