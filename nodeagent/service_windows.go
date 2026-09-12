//go:build windows

package nodeagent

import (
	"context"
	"golang.org/x/sys/windows/svc"
	"os"
	"os/exec"
	"path/filepath"
)

type serviceHandler struct{ version string }

func (h serviceHandler) Execute(_ []string, requests <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	status <- svc.Status{State: svc.StartPending}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		cfg, err := LoadConfig(DefaultConfigPath())
		if err == nil {
			_ = Run(ctx, cfg, h.version)
		}
		close(done)
	}()
	status <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for {
		select {
		case req := <-requests:
			if req.Cmd == svc.Stop || req.Cmd == svc.Shutdown {
				status <- svc.Status{State: svc.StopPending}
				cancel()
				<-done
				return false, 0
			}
		case <-done:
			return false, 1
		}
	}
}
func MaybeRunService(version string) (bool, error) {
	is, err := svc.IsWindowsService()
	if err != nil || !is {
		return false, err
	}
	return true, svc.Run("DomainSpriteNode", serviceHandler{version})
}
func DefaultConfigPath() string {
	return filepath.Join(os.Getenv("ProgramData"), "DomainSprite", "node.json")
}
func InstallService(config string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	dir := filepath.Join(os.Getenv("ProgramFiles"), "DomainSprite Node")
	if err = os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	target := filepath.Join(dir, "DomainSpriteNode.exe")
	data, err := os.ReadFile(exe)
	if err != nil {
		return err
	}
	if err = os.WriteFile(target, data, 0755); err != nil {
		return err
	}
	// Remove inherited access and grant only SYSTEM and Administrators access
	// to the identity/configuration file.
	if err = exec.Command("icacls.exe", config, "/inheritance:r", "/grant:r", "SYSTEM:F", "Administrators:F").Run(); err != nil {
		return err
	}
	bin := "\"" + target + "\" run"
	if err = exec.Command("sc.exe", "create", "DomainSpriteNode", "binPath=", bin, "start=", "auto").Run(); err != nil {
		return err
	}
	_ = exec.Command("sc.exe", "failure", "DomainSpriteNode", "reset=", "0", "actions=", "restart/5000").Run()
	return exec.Command("sc.exe", "start", "DomainSpriteNode").Run()
}
func ServiceAction(action string) error {
	switch action {
	case "status":
		return exec.Command("sc.exe", "query", "DomainSpriteNode").Run()
	case "restart":
		_ = exec.Command("sc.exe", "stop", "DomainSpriteNode").Run()
		return exec.Command("sc.exe", "start", "DomainSpriteNode").Run()
	case "uninstall":
		_ = exec.Command("sc.exe", "stop", "DomainSpriteNode").Run()
		return exec.Command("sc.exe", "delete", "DomainSpriteNode").Run()
	}
	return nil
}
