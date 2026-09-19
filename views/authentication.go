package views

import (
	"DDNSServer/db"
	"DDNSServer/models/requestModel"
	"crypto/subtle"
	"github.com/gin-gonic/gin"
)

func secureEqual(a, b string) bool { return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }

func ApiAuthentication(c *gin.Context) {
	IdentityAuthentication(c)
}

func FastAuthentication(c *gin.Context) {
	accessSalt := c.GetHeader("AccessSalt")
	if accessSalt == "" || !secureEqual(accessSalt, db.CurrentFastConfig().AccessSalt) {
		requestModel.Forbidden(c, "AccessSalt is error")
		c.Abort()
		return
	}
}
