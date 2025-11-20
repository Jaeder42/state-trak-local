package main

import (
	"fmt"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"jaeder42.tech/state-trak-local/controllers"
)

func main() {
	router := gin.Default()
	router.Use(cors.Default())
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	router.GET("/:id", controllers.GetRound)
	err := router.Run(":3001")
	if err != nil {
		fmt.Println(err)
	}
	// controllers.GetGame(0, 100)
}
