package controllers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func checkErr(err error) {
	if err != nil {
		fmt.Println(err)
	}
}

func GetRound(c *gin.Context) {
	demoId := c.Param("id")
	round := c.Param("round")
	filename := filepath.Join(demoDir(demoId), filepath.Base(round)+".json")
	if _, err := os.Stat(filename); err != nil {
		c.JSON(404, gin.H{"error": "round not found"})
		return
	}
	c.File(filename)
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

func GetRounds(c *gin.Context) {
	demoId := c.Param("id")
	dir := demoDir(demoId)
	entries, err := os.ReadDir(dir)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	var summaries []RoundSummary
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") || name == "output.json" {
			continue
		}
		num, err := strconv.Atoi(strings.TrimSuffix(name, ".json"))
		if err != nil {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		var round struct {
			Round   *int          `json:"round"`
			Winner  string        `json:"winner"`
			Economy *RoundEconomy `json:"economy"`
		}
		if err := json.Unmarshal(data, &round); err != nil {
			continue
		}
		roundNum := num
		if round.Round != nil {
			roundNum = *round.Round
		}
		summary := RoundSummary{Round: roundNum, Winner: round.Winner}
		if round.Economy != nil {
			if round.Economy.CT != nil {
				summary.CTBuy = round.Economy.CT.Type
				for _, p := range round.Economy.CT.Players {
					summary.CTSteamIds = append(summary.CTSteamIds, p.SteamId)
				}
			}
			if round.Economy.T != nil {
				summary.TBuy = round.Economy.T.Type
				for _, p := range round.Economy.T.Players {
					summary.TSteamIds = append(summary.TSteamIds, p.SteamId)
				}
			}
		}
		summaries = append(summaries, summary)
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Round < summaries[j].Round
	})

	c.JSON(200, summaries)
}
