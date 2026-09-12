package views

import "testing"

func TestMaskCredential(t *testing.T) {
	for _, v := range []string{"", "a", "abcd"} {
		if got := maskCredential(v); got != "********" {
			t.Fatalf("maskCredential(%q)=%q", v, got)
		}
	}
	if got := maskCredential("abcdef"); got != "abcd********" {
		t.Fatal(got)
	}
}
