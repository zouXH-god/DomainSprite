package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestUnknownAuthRouteDoesNotFallbackToSPA(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	Register(router)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/auth/unknown", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d", recorder.Code)
	}
	if !strings.Contains(recorder.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("content type = %q", recorder.Header().Get("Content-Type"))
	}
}
