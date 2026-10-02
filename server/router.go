package server

// The HTTP surface shared by the portable binary (main.go) and the Wails
// desktop app (desktop/): one gin router with the API routes and the
// embedded SPA fallback. Runs in-process for the local app only — there is
// no deployment/hosting story.

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"jaeder42.tech/state-trak-local/controllers"
	"jaeder42.tech/state-trak-local/web"
)

func NewRouter() *gin.Engine {
	router := gin.Default()
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	router.POST("/upload", controllers.UploadDemo)
	router.GET("/demos", controllers.ListDemos)
	router.GET("/demos/:id/status", controllers.GetDemoStatus)
	router.DELETE("/demos/:id", controllers.DeleteDemo)
	router.GET("/demos/:id/output", controllers.GetOutput)
	router.GET("/demos/:id/rounds", controllers.GetRounds)
	router.GET("/demos/:id/analysis", controllers.GetAnalysis)
	router.GET("/demos/:id/postplant", controllers.GetPostPlant)
	router.POST("/demos/:id/coach", controllers.GetCoach)
	router.GET("/demos/:id/:round", controllers.GetRound)

	// Everything else serves the embedded React app (SPA fallback).
	router.NoRoute(web.Handler())
	return router
}
