package views

import (
	"DDNSServer/models"
	"github.com/gin-gonic/gin"
)

func ApiAuthentication(c *gin.Context) {
	accessKeyId := c.GetHeader("AccessKeyId")
	accessKeySecret := c.GetHeader("AccessKeySecret")
	if accessKeyId == "" || accessKeySecret == "" {
		c.JSON(400, gin.H{
			"message": "AccessKeyId or AccessKeySecret is empty",
		})
		c.Abort()
		return
	}
	if accessKeyId != models.AccountConfig.BaseConfig.AccessKeyId || accessKeySecret != models.AccountConfig.BaseConfig.AccessKeySecret {
		c.JSON(400, gin.H{
			"message": "AccessKeyId or AccessKeySecret is error",
		})
		c.Abort()
		return
	}
}

func FastAuthentication(c *gin.Context) {
	accessSalt := c.GetHeader("AccessSalt")
	if accessSalt == "" || accessSalt != models.AccountConfig.FastConfig.AccessSalt {
		c.JSON(400, gin.H{
			"message": "AccessSalt is error",
		})
		c.Abort()
		return
	}
}
