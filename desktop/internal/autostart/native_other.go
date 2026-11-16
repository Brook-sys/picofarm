//go:build !linux && !darwin && !windows

package autostart

import "errors"

func nativeManager() (Manager, error) {
	return nil, errors.New("autostart is unsupported on this operating system")
}
