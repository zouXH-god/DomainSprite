package main

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRoutesCanBeRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerRoutes(r)
}
