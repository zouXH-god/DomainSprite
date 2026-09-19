package views

import (
	"DDNSServer/models"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestFastRecordViewDoesNotExposeToken(t *testing.T) {
	payload := models.FastData{Token: "should-never-leak", RecordInfo: models.RecordInfo{
		Id: "record-1", DomainName: "example.com", RecordName: "host", RecordContent: "192.0.2.1", RecordType: "A",
	}}
	b, err := json.Marshal(toFastRecordView(fastRecordRow("test", payload.RecordInfo, "hash", "encrypted")))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), payload.Token) || strings.Contains(strings.ToLower(string(b)), "token") {
		t.Fatalf("management response exposes token: %s", b)
	}
	if !strings.Contains(string(b), `"fqdn":"host.example.com"`) {
		t.Fatalf("unexpected management response: %s", b)
	}
}

func TestFastPageParams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		query    string
		wantPage int
		wantSize int
		wantErr  bool
	}{
		{"", 1, 20, false},
		{"?page=2&pageSize=100", 2, 100, false},
		{"?page=0", 0, 0, true},
		{"?pageSize=101", 0, 0, true},
		{"?page=invalid", 0, 0, true},
	}
	for _, tc := range tests {
		req := httptest.NewRequest("GET", "/records"+tc.query, nil)
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = req
		page, size, err := fastPageParams(ctx)
		if (err != nil) != tc.wantErr || page != tc.wantPage || size != tc.wantSize {
			t.Errorf("query %q: got (%d, %d, %v)", tc.query, page, size, err)
		}
	}
}

func TestValidFastRecordName(t *testing.T) {
	for _, valid := range []string{"@", "host-001", "api.dev", "_acme-challenge"} {
		if !validFastRecordName(valid) {
			t.Errorf("expected valid name %q", valid)
		}
	}
	for _, invalid := range []string{"", ".host", "host.", "host/name", "host name"} {
		if validFastRecordName(invalid) {
			t.Errorf("expected invalid name %q", invalid)
		}
	}
}
