//go:build windows

package autostart

import (
	"errors"
	"os"

	"golang.org/x/sys/windows/registry"
)

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const ownershipKey = `Software\Picofarm\Autostart`
const valueName = "PicofarmDesktop"

type windowsStore struct{}

func registryPath(ownership bool) string {
	if ownership {
		return ownershipKey
	}
	return runKey
}
func missingRegistry(err error) error {
	if errors.Is(err, registry.ErrNotExist) {
		return os.ErrNotExist
	}
	return err
}
func (windowsStore) Get(ownership bool) (string, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, registryPath(ownership), registry.QUERY_VALUE)
	if err != nil {
		return "", missingRegistry(err)
	}
	defer key.Close()
	value, _, err := key.GetStringValue(valueName)
	return value, missingRegistry(err)
}
func (windowsStore) Put(ownership bool, value string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, registryPath(ownership), registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	return key.SetStringValue(valueName, value)
}
func (windowsStore) Delete(ownership bool) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, registryPath(ownership), registry.SET_VALUE)
	if err != nil {
		return missingRegistry(err)
	}
	defer key.Close()
	return missingRegistry(key.DeleteValue(valueName))
}
func nativeManager() (Manager, error) { return NewRegistryManager(windowsStore{}), nil }
