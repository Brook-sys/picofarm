// Package autostart manages optional, per-user login startup without invoking a
// shell. Importing the package never changes startup settings.
package autostart

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"
)

// Manager controls one Picofarm-owned startup entry. Enabled reports registered
// intent, not whether the OS or a user policy will actually launch the app.
type Manager interface {
	Set(enabled bool, executable string) error
	Enabled() (bool, error)
}

var ErrNotOwned = errors.New("autostart entry is not owned by Picofarm")
var operationMu sync.Mutex

// Set changes startup only when explicitly called. Disabling ignores executable.
func Set(enabled bool, executable string) error {
	m, err := nativeManager()
	if err != nil {
		return err
	}
	return m.Set(enabled, executable)
}

// Enabled reads startup registration without creating directories or entries.
func Enabled() (bool, error) {
	m, err := nativeManager()
	if err != nil {
		return false, err
	}
	return m.Enabled()
}

func validateExecutable(executable string) error {
	if !filepath.IsAbs(executable) || strings.IndexFunc(executable, unicode.IsControl) >= 0 {
		return errors.New("autostart executable must be an absolute path without control characters")
	}
	info, err := os.Stat(executable)
	if err != nil {
		return fmt.Errorf("autostart executable: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("autostart executable must be a regular file")
	}
	return nil
}
