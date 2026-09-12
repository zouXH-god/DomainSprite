package models

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestFastStoreConcurrentWritesAndReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fast.json")
	s, err := NewFastStore(path, 1)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := s.WithWrite(func(d *FastDataJson) error {
				d.LastId++
				d.DataList = append(d.DataList, FastData{Token: string(rune('a' + i))})
				return nil
			}); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	reloaded, err := NewFastStore(path, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.LastID(); got != 26 {
		t.Fatalf("LastId=%d, want 26", got)
	}
}

func TestFastStoreRejectsCorruptJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fast.json")
	if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewFastStore(path, 1); err == nil {
		t.Fatal("expected corrupt JSON error")
	}
}
func TestNewFastToken(t *testing.T) {
	a, err := NewFastToken()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := NewFastToken()
	if len(a) != 64 || a == b {
		t.Fatalf("invalid random tokens")
	}
}
