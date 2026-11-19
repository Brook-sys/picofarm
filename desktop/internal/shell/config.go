// Package desktop contains the headless-testable desktop host boundary.
package desktop

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Mode                  string          `json:"mode"`
	RemoteURL             string          `json:"remoteURL,omitempty"`
	RemoteToken           string          `json:"remoteToken,omitempty"`
	AllowInsecureLoopback bool            `json:"allowInsecureLoopback"`
	CloseToTray           bool            `json:"closeToTray"`
	Notifications         map[string]bool `json:"notifications"`
	Autostart             bool            `json:"autostart"`
}

func DefaultConfig() Config {
	return Config{Mode: "local", CloseToTray: true, Notifications: map[string]bool{"finished": true, "failed": true, "critical": true}}
}

// ValidateRemote permits HTTPS and explicit insecure literal-loopback development
// targets only. User info, fragments and base paths are rejected: API routes are
// always rooted at / and credentials never belong in a URL.
func ValidateRemote(raw string, insecureLoopback bool) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || u.Opaque != "" {
		return nil, errors.New("remote URL must be an origin without credentials, path, query or fragment")
	}
	ip := net.ParseIP(u.Hostname())
	loopback := ip != nil && ip.IsLoopback()
	if u.Scheme != "https" && !(u.Scheme == "http" && insecureLoopback && loopback) {
		return nil, errors.New("remote URL requires HTTPS (HTTP requires explicit literal-loopback permission)")
	}
	if strings.ContainsAny(raw, "\r\n\\") {
		return nil, errors.New("invalid remote URL")
	}
	u.Path = ""
	return u, nil
}

func (c Config) Validate() error {
	if c.Mode != "local" && c.Mode != "remote" {
		return errors.New("mode must be local or remote")
	}
	if c.Mode == "remote" {
		if _, err := ValidateRemote(c.RemoteURL, c.AllowInsecureLoopback); err != nil {
			return err
		}
		// An empty token remains valid for servers that intentionally disable
		// authentication; the desktop gateway cannot enforce remote server auth.
		if strings.ContainsAny(c.RemoteToken, "\r\n") {
			return errors.New("invalid token")
		}
	}
	return nil
}

func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "picofarm-desktop"), nil
}
func LoadConfig(dir string) (Config, error) {
	c := DefaultConfig()
	b, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err = json.Unmarshal(b, &c); err != nil {
		return c, err
	}
	return c, c.Validate()
}
func SaveConfig(dir string, c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return privateWrite(dir, "config.json", b)
}
func privateWrite(dir, name string, b []byte) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".private-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(tmp, filepath.Join(dir, name))
}
func RandomSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// EncryptionKey is deliberately a private-file fallback, not an OS keychain.
// Call only after the single-instance lock is held, before constructing services.
func EncryptionKey(dir string) (string, error) {
	p := filepath.Join(dir, "encryption.key")
	b, err := os.ReadFile(p)
	if err == nil {
		if len(b) != 64 {
			return "", errors.New("invalid persisted encryption key")
		}
		if _, err = hex.DecodeString(string(b)); err != nil {
			return "", errors.New("invalid persisted encryption key")
		}
		if err = os.Chmod(p, 0600); err != nil {
			return "", err
		}
		return string(b), nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	key, err := RandomSecret()
	if err != nil {
		return "", err
	}
	if err = privateWrite(dir, "encryption.key", []byte(key)); err != nil {
		return "", fmt.Errorf("persist encryption key: %w", err)
	}
	return key, nil
}
