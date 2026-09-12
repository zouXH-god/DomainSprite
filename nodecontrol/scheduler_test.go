package nodecontrol

import "testing"

func TestSupportsProviders(t *testing.T) {
	if !supports("Ali,Tencent,Cloudflare", map[string]bool{"ali": true, "cloudflare": true}) {
		t.Fatal("expected provider set to be supported")
	}
	if supports("Ali,Tencent", map[string]bool{"cloudflare": true}) {
		t.Fatal("missing provider must reject node")
	}
}
