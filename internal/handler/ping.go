package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Ping returns a minimal JSON response confirming the v1 API is reachable.
func Ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"version": "v1",
		"status":  "ok",
	})
}
