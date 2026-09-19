package views

import (
	"strings"
	"testing"
)

func TestCertificateArchiveName(t *testing.T) {
	tests := []struct {
		name    string
		domains string
		want    string
	}{
		{name: "single", domains: "example.com", want: "example.com.zip"},
		{name: "apex and wildcard", domains: "example.com,*.example.com", want: "example.com_wildcard.example.com.zip"},
		{name: "normalizes and deduplicates", domains: " EXAMPLE.COM.,example.com", want: "example.com.zip"},
		{name: "empty fallback", domains: " , ", want: "certificate.zip"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := certificateArchiveName(tt.domains); got != tt.want {
				t.Fatalf("certificateArchiveName(%q)=%q want %q", tt.domains, got, tt.want)
			}
		})
	}
}

func TestCertificateArchiveNameLimitsLongSANList(t *testing.T) {
	domains := make([]string, 0, 100)
	for i := 0; i < 100; i++ {
		domains = append(domains, strings.Repeat("a", 20)+string(rune('a'+i%26))+".example.com")
	}
	name := certificateArchiveName(strings.Join(domains, ","))
	if len(name) > 200 || !strings.HasSuffix(name, ".zip") || !strings.Contains(name, "-more") {
		t.Fatalf("unexpected long archive name %q (length %d)", name, len(name))
	}
}
