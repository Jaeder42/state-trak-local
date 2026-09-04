package main

import (
	"flag"
	"fmt"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"jaeder42.tech/state-trak-local/controllers"
	"jaeder42.tech/state-trak-local/web"
)

func main() {
	parse := flag.String("parse", "", "parse the given .dem file and exit (writes JSON to controllers/data/output/local)")
	flag.Parse()

	if *parse != "" {
		controllers.ParseDemo("local", *parse)
		return
	}

	router := gin.Default()
	router.Use(cors.Default())
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
	router.GET("/demos/:id/:round", controllers.GetRound)

	router.NoRoute(web.Handler())
	if err := router.Run(":3001"); err != nil {
		fmt.Println(err)
	}
}
