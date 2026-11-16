package autostart

import (
	"errors"
	"os"
	"strings"
)

// RegistryStore is an injectable per-user Windows registry adapter. Get returns
// os.ErrNotExist for a missing value. Implementations must not execute commands.
// The ownership record lives outside Run and is never itself launched.
type RegistryStore interface {
	Get(ownership bool) (string, error)
	Put(ownership bool, value string) error
	Delete(ownership bool) error
}

var errMissing = os.ErrNotExist

type registryManager struct{ store RegistryStore }

// NewRegistryManager permits isolated tests without touching the host registry.
func NewRegistryManager(store RegistryStore) Manager { return &registryManager{store} }

func (m *registryManager) owned() (bool, error) {
	command, err := m.store.Get(false)
	if errors.Is(err, errMissing) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	owner, err := m.store.Get(true)
	if errors.Is(err, errMissing) {
		return false, ErrNotOwned
	}
	if err != nil {
		return false, err
	}
	if command != owner || command == "" {
		return false, ErrNotOwned
	}
	return true, nil
}

func (m *registryManager) Enabled() (bool, error) {
	operationMu.Lock()
	defer operationMu.Unlock()
	return m.owned()
}

func (m *registryManager) Set(enabled bool, executable string) error {
	operationMu.Lock()
	defer operationMu.Unlock()
	if enabled {
		if err := validateExecutable(executable); err != nil {
			return err
		}
		// Windows filenames cannot contain quotes. Do not permit argument injection
		// even when this portable backend is exercised from another platform.
		if strings.ContainsRune(executable, '"') {
			return errors.New("invalid Windows executable path")
		}
	}
	exists, err := m.owned()
	if err != nil {
		return err
	}
	if !enabled {
		if !exists {
			return nil
		}
		if err := m.store.Delete(false); err != nil && !errors.Is(err, errMissing) {
			return err
		}
		if err := m.store.Delete(true); err != nil && !errors.Is(err, errMissing) {
			return err
		}
		return nil
	}
	command := "\"" + executable + "\""
	// Write Run first. If recording ownership fails, preserve the previous value
	// so a partial operation does not silently take ownership of another entry.
	previous := ""
	if exists {
		previous, err = m.store.Get(false)
		if err != nil {
			return err
		}
	}
	if err := m.store.Put(false, command); err != nil {
		return err
	}
	if err := m.store.Put(true, command); err != nil {
		var rollback error
		if exists {
			rollback = m.store.Put(false, previous)
		} else {
			rollback = m.store.Delete(false)
		}
		return errors.Join(err, rollback)
	}
	return nil
}
