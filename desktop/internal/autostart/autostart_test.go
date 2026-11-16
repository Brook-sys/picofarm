package autostart

import (
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func executableFixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("test fixture"), 0700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFileLifecycle(t *testing.T) {
	for _, platform := range []string{"linux", "darwin"} {
		t.Run(platform, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "not-created")
			manager, err := NewFileManager(platform, root)
			if err != nil {
				t.Fatal(err)
			}
			m := manager.(*fileManager)
			if on, err := m.Enabled(); err != nil || on {
				t.Fatalf("initial: %v %v", on, err)
			}
			if err := m.Set(false, "ignored"); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("read/disable created config: %v", err)
			}
			executable := executableFixture(t, "Picofarm & <app> $cash %field% `tick`")
			for i := 0; i < 2; i++ {
				if err := m.Set(true, executable); err != nil {
					t.Fatal(err)
				}
				if on, err := m.Enabled(); err != nil || !on {
					t.Fatalf("enabled: %v %v", on, err)
				}
			}
			data, err := os.ReadFile(m.path)
			if err != nil {
				t.Fatal(err)
			}
			if platform == "darwin" {
				decoder := xml.NewDecoder(strings.NewReader(string(data)))
				found := false
				for {
					token, err := decoder.Token()
					if err == io.EOF {
						break
					}
					if err != nil {
						t.Fatal(err)
					}
					if text, ok := token.(xml.CharData); ok && string(text) == executable {
						found = true
					}
				}
				if !found {
					t.Fatalf("XML did not preserve executable: %s", data)
				}
				if !strings.Contains(string(data), "&amp;") || !strings.Contains(string(data), "&lt;") {
					t.Fatal("missing XML escaping")
				}
			} else {
				if !strings.Contains(string(data), "%%field%%") || !strings.Contains(string(data), `\\$cash`) || !strings.Contains(string(data), `\\`+"`tick") {
					t.Fatalf("missing desktop escaping: %s", data)
				}
			}
			if err := m.Set(false, ""); err != nil {
				t.Fatal(err)
			}
			if on, err := m.Enabled(); err != nil || on {
				t.Fatalf("disabled: %v %v", on, err)
			}
		})
	}
}

func TestFileRefusesUnownedAndSymlink(t *testing.T) {
	for _, platform := range []string{"linux", "darwin"} {
		t.Run(platform, func(t *testing.T) {
			manager, _ := NewFileManager(platform, t.TempDir())
			m := manager.(*fileManager)
			if err := os.MkdirAll(filepath.Dir(m.path), 0700); err != nil {
				t.Fatal(err)
			}
			foreign := []byte("foreign entry\n")
			if err := os.WriteFile(m.path, foreign, 0600); err != nil {
				t.Fatal(err)
			}
			executable := executableFixture(t, "app")
			for _, enable := range []bool{true, false} {
				if err := m.Set(enable, executable); !errors.Is(err, ErrNotOwned) {
					t.Fatalf("unowned Set: %v", err)
				}
			}
			if _, err := m.Enabled(); !errors.Is(err, ErrNotOwned) {
				t.Fatalf("unowned Enabled: %v", err)
			}
			data, _ := os.ReadFile(m.path)
			if string(data) != string(foreign) {
				t.Fatal("foreign entry modified")
			}
			if err := os.Remove(m.path); err != nil {
				t.Fatal(err)
			}
			target := executableFixture(t, "target")
			if err := os.Symlink(target, m.path); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
			if err := m.Set(false, ""); !errors.Is(err, ErrNotOwned) {
				t.Fatalf("symlink: %v", err)
			}
		})
	}
}

func TestInvalidExecutable(t *testing.T) {
	manager, _ := NewFileManager("linux", t.TempDir())
	for _, path := range []string{"", "relative", "/missing/picofarm", t.TempDir(), "/invalid\npath", "/invalid\x00path"} {
		if err := manager.Set(true, path); err == nil {
			t.Errorf("accepted %q", path)
		}
	}
	if _, err := NewFileManager("linux", "relative"); err == nil {
		t.Fatal("relative config accepted")
	}
}

func TestDesktopEscaping(t *testing.T) {
	m := &fileManager{platform: "linux"}
	got := string(m.content("/a b/quote\"slash\\dollar$percent%tick`"))
	want := "Exec=\"/a b/quote\\\\\"slash\\\\\\\\dollar\\\\$percent%%tick\\\\`\"\n"
	if !strings.Contains(got, want) {
		t.Fatalf("got %q, expected line %q", got, want)
	}
}

type memoryRegistry struct {
	values        map[bool]string
	failOwnership bool
}

func (s *memoryRegistry) Get(owner bool) (string, error) {
	value, ok := s.values[owner]
	if !ok {
		return "", os.ErrNotExist
	}
	return value, nil
}
func (s *memoryRegistry) Put(owner bool, value string) error {
	if owner && s.failOwnership {
		return errors.New("injected failure")
	}
	s.values[owner] = value
	return nil
}
func (s *memoryRegistry) Delete(owner bool) error { delete(s.values, owner); return nil }

func TestRegistryLifecycleAndOwnership(t *testing.T) {
	store := &memoryRegistry{values: map[bool]string{}}
	m := NewRegistryManager(store)
	if on, err := m.Enabled(); err != nil || on {
		t.Fatalf("initial: %v %v", on, err)
	}
	if err := m.Set(false, ""); err != nil {
		t.Fatal(err)
	}
	executable := executableFixture(t, "app with spaces")
	if err := m.Set(true, executable); err != nil {
		t.Fatal(err)
	}
	if store.values[false] != "\""+executable+"\"" {
		t.Fatal("unquoted command")
	}
	if on, err := m.Enabled(); err != nil || !on {
		t.Fatalf("enabled: %v %v", on, err)
	}
	store.values[false] = "foreign"
	if err := m.Set(false, ""); !errors.Is(err, ErrNotOwned) {
		t.Fatalf("foreign: %v", err)
	}
	if store.values[false] != "foreign" {
		t.Fatal("foreign value modified")
	}
	store.values[false] = store.values[true]
	if err := m.Set(false, ""); err != nil {
		t.Fatal(err)
	}
	if len(store.values) != 0 {
		t.Fatal("registration remains")
	}
	store.failOwnership = true
	if err := m.Set(true, executable); err == nil {
		t.Fatal("expected write error")
	}
	if len(store.values) != 0 {
		t.Fatal("Run registration not rolled back")
	}
}
