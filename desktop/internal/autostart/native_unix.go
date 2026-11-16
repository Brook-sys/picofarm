//go:build linux || darwin

package autostart

import (
	"os"
	"path/filepath"
	"runtime"
)

func nativeManager() (Manager, error) {
	if runtime.GOOS == "linux" {
		config, err := os.UserConfigDir()
		if err != nil {
			return nil, err
		}
		return NewFileManager("linux", config)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return NewFileManager("darwin", filepath.Join(home, "Library"))
}
