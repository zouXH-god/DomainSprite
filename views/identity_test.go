package views

import (
	"DDNSServer/db"
	"DDNSServer/models"
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestSetupTokenCreatesOnlyFirstAdmin(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = database.AutoMigrate(&models.User{}, &models.AccessKey{}, &models.WebSession{}, &models.AuditLog{}); err != nil {
		t.Fatal(err)
	}
	old := db.DB
	db.DB = database
	defer func() { db.DB = old }()
	bootstrap.Lock()
	bootstrap.hash = db.HashToken("one-use")
	bootstrap.expires = time.Now().Add(time.Minute)
	bootstrap.Unlock()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/auth/setup", Setup)
	body := []byte(`{"username":"admin","password":"long-password","setupToken":"one-use"}`)
	first := httptest.NewRecorder()
	r.ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/auth/setup", bytes.NewReader(body)))
	if first.Code != http.StatusOK {
		t.Fatalf("first setup status=%d body=%s", first.Code, first.Body.String())
	}
	var user models.User
	if err = database.First(&user).Error; err != nil || user.Role != "admin" || user.PasswordHash == "long-password" {
		t.Fatalf("admin not securely created: %#v err=%v", user, err)
	}
	second := httptest.NewRecorder()
	r.ServeHTTP(second, httptest.NewRequest(http.MethodPost, "/auth/setup", bytes.NewReader(body)))
	if second.Code != http.StatusConflict {
		t.Fatalf("second setup status=%d", second.Code)
	}
}
