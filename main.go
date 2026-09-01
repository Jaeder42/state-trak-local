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
	parse := flag.Bool("parse", false, "parse the demo and write output files")
	flag.Parse()

	if *parse {
		controllers.GetGame(0, 100)
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
	router.GET("/demos/:id/output", func(c *gin.Context) {
		c.File("./controllers/data/output/" + c.Param("id") + "/output.json")
	})
	router.GET("/demos/:id/rounds", controllers.GetRounds)
	router.GET("/demos/:id/:round", controllers.GetRound)

	router.NoRoute(web.Handler())
	err := router.Run(":3001")
	if err != nil {
		fmt.Println(err)
	}
}
