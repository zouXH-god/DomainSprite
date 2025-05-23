package views

import (
	"DDNSServer/models"
	"DDNSServer/models/requestModel"
	"github.com/gin-gonic/gin"
)

func ApiAuthentication(c *gin.Context) {
	accessKeyId := c.GetHeader("AccessKeyId")
	accessKeySecret := c.GetHeader("AccessKeySecret")
	if accessKeyId == "" || accessKeySecret == "" {
		requestModel.Forbidden(c, "AccessKeyId or AccessKeySecret is empty")
		c.Abort()
		return
	}
	if accessKeyId != models.AccountConfig.BaseConfig.AccessKeyId || accessKeySecret != models.AccountConfig.BaseConfig.AccessKeySecret {
		requestModel.Forbidden(c, "AccessKeyId or AccessKeySecret is error")
		c.Abort()
		return
	}
}

func FastAuthentication(c *gin.Context) {
	accessSalt := c.GetHeader("AccessSalt")
	if accessSalt == "" || accessSalt != models.AccountConfig.FastConfig.AccessSalt {
		requestModel.Forbidden(c, "AccessSalt is error")
		c.Abort()
		return
	}
}
