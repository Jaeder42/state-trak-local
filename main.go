package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"jaeder42.tech/state-trak-local/controllers"
	"jaeder42.tech/state-trak-local/web"
)

func main() {
	// Optional .env in the repo root (gitignored) — e.g. TYPESAFE_API_KEY.
	// Real environment variables take precedence.
	_ = godotenv.Load()

	parse := flag.String("parse", "", "parse the given .dem file and exit (writes JSON to controllers/data/output/local)")
	jev := flag.String("jev", "", "analyze a parsed demo's round economies with JEV (eco/force/full detection); argument is a demo id, e.g. -jev=local")
	flag.Parse()

	if *parse != "" {
		controllers.ParseDemo("local", *parse)
		return
	}

	if *jev != "" {
		if err := controllers.AnalyzeEconomy(*jev); err != nil {
			fmt.Println("analysis failed:", err)
			os.Exit(1)
		}
		return
	}

	router := gin.Default()
	// The client is served same-origin, so no CORS is needed by default.
	// Set ALLOWED_ORIGIN to explicitly enable a cross-origin client.
	if origin := os.Getenv("ALLOWED_ORIGIN"); origin != "" {
		router.Use(cors.New(cors.Config{
			AllowOrigins: []string{origin},
			AllowMethods: []string{"GET", "POST", "DELETE", "OPTIONS"},
			AllowHeaders: []string{"Origin", "Content-Type", "Accept"},
		}))
	}
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
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
	router.GET("/demos/:id/:round", controllers.GetRound)

	router.NoRoute(web.Handler())
	if err := router.Run(":3007"); err != nil {
		fmt.Println(err)
	}
}
