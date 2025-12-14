package jobmanager

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/google/uuid"
)

var uuidGen = uuid.NewRandom

type DevConfig struct {
	UseDevPath bool `toml:"useDevPath"`
}

func generateUUID() string {
	var UUID uuid.UUID
	UUID, err := uuidGen()
	if err != nil {
		return time.Now().Format(time.UnixDate)
	}
	return UUID.String()
}

func isTestMode() bool {
	if strings.HasSuffix(os.Args[0], ".test") {
		return true
	}
	for _, a := range os.Args {
		if strings.HasPrefix(a, "-test.") {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || !os.IsNotExist(err)
}

func findConfigDirTest(start string, maxDepth int) (string, error) {
	if fileExists(filepath.Join(start, "dev.toml")) {
		return start, nil
	}
	if fileExists(filepath.Join(start, "go.mod")) {
		return start, nil
	}
	cur := start
	for i := 0; i < maxDepth; i++ {
		next := filepath.Dir(cur)
		if next == cur {
			break
		}
		cur = next
		if fileExists(filepath.Join(cur, "go.mod")) {
			return cur, nil
		}
	}
	return "", fmt.Errorf("preferences: test mode cannot find go.mod within %d levels from %s", maxDepth, start)
}

func getDevHomeDir() string {
	if !isTestMode() {
		return ""
	}

	wd, err := os.Getwd()
	if err != nil {
		return ""
	}

	rootDir, err := findConfigDirTest(wd, 5)
	if err != nil {
		return ""
	}

	devTomlPath := filepath.Join(rootDir, "dev.toml")

	var config DevConfig
	if _, err := toml.DecodeFile(devTomlPath, &config); err != nil {
		return ""
	}

	if config.UseDevPath {
		return rootDir
	}

	return ""
}
