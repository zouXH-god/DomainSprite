//go:build windows

package utils

import (
	"fmt"
	"os/exec"
	"os/user"
)

func restrictSecretFile(path string) error {
	current, err := user.Current()
	if err != nil {
		return err
	}
	// S-1-5-18 is LocalSystem and S-1-5-32-544 is the built-in
	// Administrators group. SIDs avoid failures on localized Windows editions.
	output, err := exec.Command("icacls.exe", path, "/inheritance:r", "/grant:r", current.Username+":(F)", "*S-1-5-18:(F)", "*S-1-5-32-544:(F)").CombinedOutput()
	if err != nil {
		return fmt.Errorf("icacls: %w: %s", err, output)
	}
	return nil
}
