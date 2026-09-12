package nodeagent

import "testing"

func TestGRPCTargetRequiresHTTPS(t *testing.T) {
	if _, _, err := grpcTarget("http://localhost:2486"); err == nil {
		t.Fatal("plaintext controller URL accepted")
	}
	target, name, err := grpcTarget("https://controller.example.com:2486")
	if err != nil || target != "controller.example.com:2486" || name != "controller.example.com" {
		t.Fatalf("unexpected target: %q %q %v", target, name, err)
	}
}
