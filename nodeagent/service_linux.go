//go:build linux

package nodeagent

import (
	"fmt"
	"os"
	"os/exec"
)

func MaybeRunService(string) (bool, error) { return false, nil }
func DefaultConfigPath() string            { return "/etc/domainsprite-node/node.json" }
func InstallService(config string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	target := "/usr/local/bin/domainsprite-node"
	data, err := os.ReadFile(exe)
	if err != nil {
		return err
	}
	if err = os.WriteFile(target, data, 0755); err != nil {
		return err
	}
	unit := fmt.Sprintf("[Unit]\nDescription=DomainSprite Certificate Node\nAfter=network-online.target\n[Service]\nExecStart=%s run\nRestart=always\nRestartSec=5\n[Install]\nWantedBy=multi-user.target\n", target)
	if err = os.WriteFile("/etc/systemd/system/domainsprite-node.service", []byte(unit), 0644); err != nil {
		return err
	}
	if err = exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		return err
	}
	return exec.Command("systemctl", "enable", "--now", "domainsprite-node").Run()
}
func ServiceAction(action string) error {
	switch action {
	case "status":
		return exec.Command("systemctl", "status", "domainsprite-node").Run()
	case "restart":
		return exec.Command("systemctl", "restart", "domainsprite-node").Run()
	case "uninstall":
		_ = exec.Command("systemctl", "disable", "--now", "domainsprite-node").Run()
		if err := os.Remove("/etc/systemd/system/domainsprite-node.service"); err != nil && !os.IsNotExist(err) {
			return err
		}
		return exec.Command("systemctl", "daemon-reload").Run()
	}
	return nil
}
