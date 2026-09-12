package DDNS

import (
	"DDNSServer/models"
	"testing"
)

func TestNewBaseProviderRejectsUnknownType(t *testing.T) {
	p, err := NewBaseProvider(models.Account{Type: "unknown"})
	if err == nil || p != nil {
		t.Fatal("unknown provider must fail")
	}
}
