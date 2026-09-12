//go:build !linux && !windows

package nodeagent

import (
	"errors"
	"os"
	"path/filepath"
)

func MaybeRunService(string) (bool, error) { return false, nil }
func DefaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".domainsprite-node", "node.json")
}
func InstallService(string) error {
	return errors.New("automatic service installation is supported on Linux and Windows; use run")
}
func ServiceAction(string) error {
	return errors.New("system service is not supported on this platform")
}
