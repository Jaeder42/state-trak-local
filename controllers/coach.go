package controllers

// LLM coach analysis — "bring your own LLM".
//
// POST /demos/:id/coach with a body of
//
//	{"llm": {"baseUrl": "...", "model": "...", "apiKey": "..."}, "steamId": "..."}
//
// The server assembles the demo context (map, per-round winners + buy types,
// post-plant positioning analysis, the chosen player's sides), sends it to
// any OpenAI-compatible /chat/completions endpoint the client configured
// (OpenAI, Anthropic-compatible gateways, local Ollama at
// http://localhost:11434/v1, …) and returns the coach's text.
//
// Keys are strictly per-request: they come from the client's browser settings,
// are used for this one call, and are never logged, cached or persisted.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const coachTimeout = 120 * time.Second

type LLMConfig struct {
	BaseUrl string `json:"baseUrl"`
	Model   string `json:"model"`
	ApiKey  string `json:"apiKey"`
}

type CoachRequest struct {
	LLM     LLMConfig `json:"llm"`
	SteamId string    `json:"steamId"`
}

type CoachResponse struct {
	Text  string `json:"text"`
	Model string `json:"model"`
}

// coachContext is the compact demo state sent to the LLM. Frames are way too
// big — everything here is pre-digested analysis data.
type coachContext struct {
	Map       string           `json:"map"`
	Player    coachPlayer      `json:"player"`
	Summary   PostPlantSummary `json:"postPlantSummary"`
	Rounds    []RoundSummary   `json:"rounds"`
	PostPlant []PostPlantRound `json:"postPlant,omitempty"`
}

type coachPlayer struct {
	SteamId string `json:"steamId"`
	Name    string `json:"name"`
	// Sides maps round number -> the side the player's team was on
	// ("CT"/"T"), from the round rosters.
	Sides map[string]string `json:"sides,omitempty"`
}

const coachSystemPrompt = `You are an expert Counter-Strike 2 coach reviewing a demo replay for one player's team. You get the demo's rounds (winners, buy types), a post-plant positioning analysis (T setups, CT retake entries) and the player's side per round.

Write a concise coaching review of the player's team:
- Concrete good patterns to keep and bad habits to fix, each backed by specific round numbers from the data.
- Post-plant behavior (setup spacing, distance to the bomb, holding vs pushing, retake timing and grouping) is the focus where the data supports it.
- Economy/buy discipline where notable.
- End with 2-3 actionable takeaways for the next session.
Plain text, no markdown headings, max ~350 words. Use "you"/"your team" to address the player. Do not invent data that isn't present.`

// GetCoach runs the BYO-LLM coaching analysis for a demo.
func GetCoach(c *gin.Context) {
	id := c.Param("id")
	dir := demoDir(id)
	if _, err := os.Stat(filepath.Join(dir, "output.json")); err != nil {
		c.JSON(404, gin.H{"error": "demo not found"})
		return
	}

	var req CoachRequest
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 64<<10))
	if err != nil {
		c.JSON(400, gin.H{"error": "failed to read request body"})
		return
	}
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(400, gin.H{"error": "invalid JSON body"})
		return
	}
	req.LLM.BaseUrl = strings.TrimSpace(req.LLM.BaseUrl)
	req.LLM.Model = strings.TrimSpace(req.LLM.Model)
	if req.LLM.BaseUrl == "" || req.LLM.Model == "" {
		c.JSON(400, gin.H{"error": "llm.baseUrl and llm.model are required — configure them in the app settings (🔑)"})
		return
	}

	ctx, err := buildCoachContext(dir, req.SteamId)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	text, model, err := callLLM(req.LLM, ctx)
	if err != nil {
		c.JSON(502, gin.H{"error": "LLM call failed: " + err.Error()})
		return
	}
	c.JSON(200, CoachResponse{Text: text, Model: model})
}

// buildCoachContext assembles the compact demo state for the LLM prompt.
func buildCoachContext(dir, steamId string) (*coachContext, error) {
	var out struct {
		Map     string `json:"map"`
		Players []struct {
			Name    *string `json:"name"`
			SteamID *string `json:"steamId"`
		} `json:"players"`
	}
	if data, err := os.ReadFile(filepath.Join(dir, "output.json")); err != nil {
		return nil, fmt.Errorf("demo metadata missing")
	} else if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}

	summaries, err := loadRoundSummaries(dir)
	if err != nil {
		return nil, err
	}

	// Post-plant analysis — reuse the cache if present, compute otherwise.
	var post *PostPlantAnalysis
	if data, err := os.ReadFile(filepath.Join(dir, "postplant.json")); err == nil {
		_ = json.Unmarshal(data, &post)
	}
	if post == nil {
		if computed, err := computePostPlant(dir); err == nil {
			post = computed
			if data, err := json.Marshal(post); err == nil {
				_ = os.WriteFile(filepath.Join(dir, "postplant.json"), data, 0644)
			}
		}
	}

	player := coachPlayer{SteamId: steamId}
	for _, p := range out.Players {
		if p.SteamID != nil && *p.SteamID == steamId {
			if p.Name != nil {
				player.Name = *p.Name
			}
			break
		}
	}
	player.Sides = map[string]string{}
	for _, r := range summaries {
		switch {
		case contains(r.CTSteamIds, steamId):
			player.Sides[fmt.Sprint(r.Round)] = "CT"
		case contains(r.TSteamIds, steamId):
			player.Sides[fmt.Sprint(r.Round)] = "T"
		}
	}

	ctxData := &coachContext{
		Map:       out.Map,
		Player:    player,
		Rounds:    summaries,
		PostPlant: post.Rounds,
	}
	if post != nil {
		ctxData.Summary = post.Summary
	}
	return ctxData, nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// chatMessage is one message of an OpenAI-compatible chat request.
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// callLLM posts the coach prompt + context to an OpenAI-compatible
// /chat/completions endpoint and returns the assistant text.
func callLLM(llm LLMConfig, ctx *coachContext) (string, string, error) {
	ctxJson, err := json.Marshal(ctx)
	if err != nil {
		return "", "", err
	}

	endpoint := strings.TrimRight(llm.BaseUrl, "/") + "/chat/completions"
	payload, err := json.Marshal(map[string]any{
		"model": llm.Model,
		"messages": []chatMessage{
			{Role: "system", Content: coachSystemPrompt},
			{Role: "user", Content: "Demo data (JSON):\n" + string(ctxJson)},
		},
		"max_tokens":  1200,
		"temperature": 0.4,
	})
	if err != nil {
		return "", "", err
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if llm.ApiKey != "" {
		req.Header.Set("Authorization", "Bearer "+llm.ApiKey)
	}

	client := &http.Client{Timeout: coachTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", "", err
	}
	if resp.StatusCode != http.StatusOK {
		detail := string(respBody)
		if len(detail) > 300 {
			detail = detail[:300]
		}
		return "", "", fmt.Errorf("%s: %s", resp.Status, detail)
	}

	var out struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", "", err
	}
	if len(out.Choices) == 0 || out.Choices[0].Message.Content == "" {
		return "", "", fmt.Errorf("empty response from LLM")
	}
	model := out.Model
	if model == "" {
		model = llm.Model
	}
	return out.Choices[0].Message.Content, model, nil
}
