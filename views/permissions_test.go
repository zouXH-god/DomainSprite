package views

import "testing"

func TestRecordFQDN(t *testing.T) {
	cases := map[string]string{"@": "example.com", "www": "www.example.com", "api.example.com": "api.example.com"}
	for name, want := range cases {
		if got := recordFQDN(name, "example.com"); got != want {
			t.Fatalf("%s => %s, want %s", name, got, want)
		}
	}
}
func TestPermissionRanks(t *testing.T) {
	if permissionRank("viewer") >= permissionRank("dns_editor") || permissionRank("dns_editor") >= permissionRank("certificate_manager") || permissionRank("certificate_manager") >= permissionRank("owner") {
		t.Fatal("permission hierarchy invalid")
	}
}
