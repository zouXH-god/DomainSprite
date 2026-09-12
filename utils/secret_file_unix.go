//go:build !windows

package utils

import "os"

func restrictSecretFile(path string) error { return os.Chmod(path, 0600) }
