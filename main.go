package main

import (
	"errors"
	"os"
	"runtime"

	"github.com/aeltai/hangar-genesis/internal/genesis"
	"github.com/cnrancher/hangar/pkg/utils"
	"github.com/sirupsen/logrus"
)

func main() {
	if runtime.GOOS == "windows" {
		logrus.Fatalf("unsupported OS: %v", runtime.GOOS)
	}
	root := genesis.NewRootCommand()
	if err := root.Execute(); err != nil {
		if errors.Is(err, genesis.ErrAborted) {
			os.Exit(130)
		}
		logrus.Fatal(err)
	}
	if err := os.RemoveAll(utils.HangarCacheDir()); err != nil {
		logrus.Warnf("failed to delete cache: %v", err)
	}
}
