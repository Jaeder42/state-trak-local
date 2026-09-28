package server

// The HTTP surface shared by the standalone server (main.go) and the Wails
// desktop app (desktop/): one gin router with the API routes and the
// embedded SPA fallback.

import (
	"net/http"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"jaeder42.tech/state-trak-local/controllers"
	"jaeder42.tech/state-trak-local/web"
)

func NewRouter() *gin.Engine {
	router := gin.Default()
	// The client is served same-origin, so no CORS is needed by default.
	// Set ALLOWED_ORIGIN to explicitly enable a cross-origin client.
	if origin := os.Getenv("ALLOWED_ORIGIN"); origin != "" {
		router.Use(cors.New(cors.Config{
			AllowOrigins:  []string{origin},
			AllowMethods:  []string{"GET", "POST", "DELETE", "OPTIONS"},
			AllowHeaders:  []string{"Origin", "Content-Type", "Accept", "X-TypeSafe-Key"},
			ExposeHeaders: []string{"Content-Disposition"},
		}))
	}
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
