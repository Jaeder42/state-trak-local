package controllers

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// GetRound serves one round's frames — the exact JSON the old
// <round>.json file held, decompressed from the store.
func GetRound(c *gin.Context) {
	demoId := c.Param("id")
	round, err := strconv.Atoi(c.Param("round"))
	if err != nil {
		c.JSON(404, gin.H{"error": "round not found"})
		return
	}
	data, ok, err := db.GetRoundJSON(demoId, round)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if !ok {
		c.JSON(404, gin.H{"error": "round not found"})
		return
	}
	c.Data(200, "application/json", data)
}

type RoundSummary struct {
	Round  int    `json:"round"`
	Winner string `json:"winner"`
	CTBuy  string `json:"ctBuy,omitempty"` // heuristic buy type (pistol/eco/force/hero/kept/half/full)
	TBuy   string `json:"tBuy,omitempty"`
	// Team rosters (steam ids) at freeze-time end, from the round's economy
	// snapshot. The client uses them to figure out which side "my" player was
	// on in each round — teams swap at halftime, so this can't be inferred from
	// the currently loaded round.
	CTSteamIds []string `json:"ctSteamIds,omitempty"`
	TSteamIds  []string `json:"tSteamIds,omitempty"`
}

// GetRounds serves the round summaries (winner, buys, rosters). The old
// version read every full 30MB round file for this; the store keeps it in
// columns, so this is now a small indexed query.
func GetRounds(c *gin.Context) {
	demoId := c.Param("id")
	exists, err := db.DemoExists(demoId)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if !exists {
		c.JSON(500, gin.H{"error": "demo not found"})
		return
	}
	summaries, err := db.GetRoundSummaries(demoId)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, summaries)
}
