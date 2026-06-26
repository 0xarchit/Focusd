//go:build !windows

package system

import (
	"errors"
)

func StartDaemon() (uint32, error) {
	return 0, errors.New("daemon mode is only supported on Windows")
}
