package autostart

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const ownerMarker = "Picofarm-owned-autostart-v1"
const launchLabel = "io.picofarm.desktop"

type fileManager struct{ path, platform string }

// NewFileManager is an injectable filesystem backend for packaging and tests.
// platform is "linux" or "darwin"; configDir is an absolute XDG config directory
// on Linux, or an absolute user Library directory on macOS. No I/O occurs here.
func NewFileManager(platform, configDir string) (Manager, error) {
	if !filepath.IsAbs(configDir) {
		return nil, errors.New("autostart configuration directory must be absolute")
	}
	switch platform {
	case "linux":
		return &fileManager{filepath.Join(configDir, "autostart", "io.picofarm.desktop.desktop"), platform}, nil
	case "darwin":
		return &fileManager{filepath.Join(configDir, "LaunchAgents", launchLabel+".plist"), platform}, nil
	default:
		return nil, fmt.Errorf("unsupported autostart platform %q", platform)
	}
}

func (m *fileManager) content(executable string) []byte {
	if m.platform == "darwin" {
		var escaped bytes.Buffer
		_ = xml.EscapeText(&escaped, []byte(executable))
		return []byte("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n<!-- " + ownerMarker + " -->\n<plist version=\"1.0\"><dict>\n<key>Label</key><string>" + launchLabel + "</string>\n<key>ProgramArguments</key><array><string>" + escaped.String() + "</string></array>\n<key>RunAtLoad</key><true/>\n</dict></plist>\n")
	}
	// Exec has its own quoting layer, then the desktop-entry string escaping
	// layer. Percent is a field-code introducer even inside quoted arguments.
	arg := strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "`", "\\`", "$", "\\$").Replace(executable)
	arg = strings.ReplaceAll(arg, "%", "%%")
	arg = strings.ReplaceAll("\""+arg+"\"", "\\", "\\\\")
	return []byte("[Desktop Entry]\nType=Application\nName=Picofarm\nExec=" + arg + "\nTerminal=false\nX-Picofarm-Owner=" + ownerMarker + "\n")
}

func (m *fileManager) readOwned() ([]byte, error) {
	info, err := os.Lstat(m.path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, ErrNotOwned
	}
	data, err := os.ReadFile(m.path)
	if err != nil {
		return nil, err
	}
	// Match a full marker line, never an incidental executable substring.
	marker := "X-Picofarm-Owner=" + ownerMarker + "\n"
	if m.platform == "darwin" {
		marker = "<!-- " + ownerMarker + " -->\n"
	}
	if !bytes.Contains(data, []byte("\n"+marker)) {
		return nil, ErrNotOwned
	}
	return data, nil
}

func (m *fileManager) Enabled() (bool, error) {
	operationMu.Lock()
	defer operationMu.Unlock()
	_, err := m.readOwned()
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

func (m *fileManager) Set(enabled bool, executable string) error {
	operationMu.Lock()
	defer operationMu.Unlock()
	if enabled {
		if err := validateExecutable(executable); err != nil {
			return err
		}
	}
	_, err := m.readOwned()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if !enabled {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return os.Remove(m.path)
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(m.path), ".picofarm-autostart-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(m.content(executable)); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), m.path)
}
