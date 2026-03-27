package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthCheck returns a simple JSON response indicating the service is healthy.
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}
