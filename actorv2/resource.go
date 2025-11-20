package actorv2

import (
    "embed"
)

//go:embed v2/**
var actorV2FS embed.FS

func GetActorV2Fs() embed.FS {
    return actorV2FS
}