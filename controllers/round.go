package controllers

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

func checkErr(err error) {
	if err != nil {
		fmt.Println(err)
	}
}

func GetRound(c *gin.Context) {
	id := c.Param("id")
	filename := "./controllers/data/output/" + id + ".json"
	// plan, err := os.ReadFile(filename)
	// checkErr(err)
	// fmt.Println(len(plan))
	// var data map[string]interface{}
	// err = json.Unmarshal(plan, &data)
	// checkErr(err)
	// c.JSON(200, data)
	c.File(filename)
}
