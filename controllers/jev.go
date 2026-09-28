package controllers

// JEV round analysis.
//
// Sends each round's economy snapshot (captured by the parser at freeze-time
// end) to TypeSafe's "System One" (JEV) decision endpoint and compares its
// semantic buy-type judgment with the parser's deterministic threshold
// baseline (buyType in game.go).
//
// API: POST https://api.typesafe.ai/v1/systemone
// Auth: Bearer TYPESAFE_API_KEY (https://console.typesafe.ai/settings/keys)
//
// Entry points:
//   - AnalyzeEconomy — the -jev CLI flag: prints the baseline/JEV comparison
//   - GetAnalysis    — GET /demos/:id/analysis: runs the same analysis,
//     caches it as analysis.json next to the round files (JEV is billed once
//     per demo) and serves the result as JSON
//
// Without TYPESAFE_API_KEY the CLI runs in dry mode (baseline + example
// request only) and GetAnalysis returns 503.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	jevEndpoint = "https://api.typesafe.ai/v1/systemone"
	jevModel    = "jev-latest"
)

// JevQuestion is one typed question from the System One API.
// Type is "noul" (yes/no probability), "choice" (pick one option) or
// "score" (position on ordered levels).
type JevQuestion struct {
	Type         string `json:"type"`
	Instructions string `json:"instructions"`
	Criteria     any    `json:"criteria,omitempty"`
}

// JevAnswer is the answer to one question, keyed by the same id.
type JevAnswer struct {
	Type          string             `json:"type"`
	Noul          float64            `json:"noul,omitempty"`
	Choice        string             `json:"choice,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Confidence    float64            `json:"confidence,omitempty"`
}

// JevUsage is the token usage reported per request.
type JevUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// JevResponse is the full System One response.
type JevResponse struct {
	Model   string               `json:"model"`
	Answers map[string]JevAnswer `json:"answers"`
	Usage   JevUsage             `json:"usage"`
}

// jevEvaluate sends one state + question batch to JEV using the given key
// (per-request "bring your own" key, or the env var for the CLI). Retries
// once on rate-limit / overload responses and transient network errors.
func jevEvaluate(key, state string, questions map[string]JevQuestion) (*JevResponse, error) {
	if key == "" {
		return nil, fmt.Errorf("no JEV key")
	}
	body, err := json.Marshal(map[string]any{
		"state":     state,
		"model":     jevModel,
		"questions": questions,
	})
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			time.Sleep(2 * time.Second)
		}
		req, err := http.NewRequest("POST", jevEndpoint, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+key)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		switch resp.StatusCode {
		case http.StatusOK:
			var out JevResponse
			if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
				resp.Body.Close()
				return nil, fmt.Errorf("jev: decode response: %w", err)
			}
			resp.Body.Close()
			return &out, nil
		case http.StatusTooManyRequests, 529:
			lastErr = fmt.Errorf("jev: server busy (HTTP %d)", resp.StatusCode)
			resp.Body.Close()
			continue
		default:
			data, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("jev: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
		}
	}
	return nil, lastErr
}

// economyQuestions builds the per-round question batch: one buy-type choice
// and one eco noul per team.
func economyQuestions() map[string]JevQuestion {
	criteria := map[string]string{
		"pistol": "pistol round: the first round of a half, everyone starts with $800 and a free pistol",
		"eco":    "saving round: under ~$1000 spent per player — pistols and light utility at most, usually no armor — keeping money for a future full buy",
		"force":  "force buy: spending most of a small bank on cheap weapons — SMGs or pistols like the Deagle, usually with armor — roughly $1000-$3000 spent per player",
		"hero":   "hero buy: exactly one player spends big on a weapon (a rifle, or a pistol like the Deagle with armor) while the rest of the team saves",
		"kept":   "no buy needed: the team already held rifles from surviving the previous round — little to no spending, maybe armor or utility top-ups",
		"half":   "partial buy: only some players hold rifles (roughly 2-3 of 5), the others save or run cheaper weapons like SMGs",
		"full":   "full buy: the whole team holds rifles (bought this round or kept from the last one), usually with armor and utility",
	}
	ecoNoul := func(side string) JevQuestion {
		return JevQuestion{
			Type:         "noul",
			Instructions: fmt.Sprintf("Did the %s team deliberately play an eco (saving) round? An eco means spending under ~$1000 per player (light utility is fine) to build money for a future full buy. Pistol rounds and rounds where the team simply kept weapons by surviving the previous round are not ecos.", side),
		}
	}
	return map[string]JevQuestion{
		"ct_buy": {
			Type:         "choice",
			Instructions: "Which buy type best describes the CT team this round?",
			Criteria:     criteria,
		},
		"t_buy": {
			Type:         "choice",
			Instructions: "Which buy type best describes the T team this round?",
			Criteria:     criteria,
		},
		"ct_eco": ecoNoul("CT"),
		"t_eco":  ecoNoul("T"),
	}
}

// lossBonus is CS2's consecutive-loss bonus: $1400 for the first loss, +$500
// per additional consecutive loss, capped at $3400.
func lossBonus(streak int) int {
	if streak < 1 {
		return 0
	}
	if streak > 5 {
		streak = 5
	}
	return 1400 + 500*(streak-1)
}

// buildEconomyState renders one round's economy as the natural-language state
// sent to JEV. Only what was known at buy time is included — the round's own
// outcome is deliberately left out so it can't bias the classification.
func buildEconomyState(mapName string, roundIdx int, scoreCT, scoreT, ctStreak, tStreak int, recentWinners []string, econ *RoundEconomy) string {
	pistol := (econ.CT != nil && econ.CT.Type == "pistol") || (econ.T != nil && econ.T.Type == "pistol")
	var b strings.Builder
	fmt.Fprintf(&b, "Counter-Strike 2 match on %s, round %d (rounds are 0-indexed).\n", mapName, roundIdx)
	fmt.Fprintf(&b, "Score before this round: CT %d - %d T. ", scoreCT, scoreT)
	if ctStreak > 0 {
		fmt.Fprintf(&b, "CT has lost the last %d round(s); their loss bonus if they lose again is $%d. ", ctStreak, lossBonus(ctStreak))
	} else if tStreak > 0 {
		fmt.Fprintf(&b, "T has lost the last %d round(s); their loss bonus if they lose again is $%d. ", tStreak, lossBonus(tStreak))
	}
	fmt.Fprintf(&b, "\n")
	if len(recentWinners) > 0 {
		fmt.Fprintf(&b, "Round winners before this one (oldest to newest): %s.\n", strings.Join(recentWinners, ", "))
	}
	fmt.Fprintf(&b, "Reference prices: rifles $2700-$3100, armor+helmet $1000, defuse kit $400, SMGs $1050-$1800, Deagle $700.\n")
	if pistol {
		fmt.Fprintf(&b, "This is a pistol round — the first round of a half: everyone starts with $800 and a free pistol; weapons do not carry over.\n")
	}
	fmt.Fprintf(&b, "\n")
	writeSideState(&b, "CT", econ.CT, roundIdx, pistol)
	writeSideState(&b, "T", econ.T, roundIdx, pistol)
	return b.String()
}

func writeSideState(b *strings.Builder, side string, t *TeamEconomy, roundIdx int, pistol bool) {
	if t == nil {
		return
	}
	fmt.Fprintf(b, "%s team economy, captured at freeze-time end, after buying:\n", side)
	if roundIdx > 0 && !pistol {
		if t.Survivors > 0 {
			fmt.Fprintf(b, "%d of %d %s players survived the previous round and kept their weapons.\n", t.Survivors, len(t.Players), side)
		} else {
			fmt.Fprintf(b, "No %s players survived the previous round; weapons did not carry over.\n", side)
		}
	}
	for _, p := range t.Players {
		desc := []string{}
		if p.Armor > 0 {
			desc = append(desc, "armor")
		}
		if p.Helmet {
			desc = append(desc, "helmet")
		}
		if p.DefuseKit {
			desc = append(desc, "defuse kit")
		}
		equip := ""
		if len(desc) > 0 {
			equip = fmt.Sprintf(" (%s)", strings.Join(desc, ", "))
		}
		holding := ""
		if p.Weapon != "" {
			holding = fmt.Sprintf(" holding %s,", p.Weapon)
		}
		fmt.Fprintf(b, "- %s:%s started with $%d, spent $%d, $%d left in bank, equipment value $%d%s\n",
			p.Name, holding, p.StartMoney, p.Spent, p.Bank, p.EquipValue, equip)
	}
	fmt.Fprintf(b, "%s totals: average equipment value $%d, total spent $%d, average spent per player $%d, %d of %d players hold rifles.\n\n", side, t.AvgEquip, t.TotalSpent, t.AvgSpent, t.Rifles, len(t.Players))
}

// slimRound is the subset of a round file the analyzer needs — the frames are
// skipped entirely.
type slimRound struct {
	Winner  string        `json:"winner"`
	Economy *RoundEconomy `json:"economy"`
}

// jevReport is the per-round analysis detail used by the CLI printer.
type jevReport struct {
	num      int
	winner   string
	econ     *RoundEconomy
	state    string
	jev      *JevResponse
	jevError error
}

// TeamAnalysis is one team's buy classification for a round: the parser's
// deterministic baseline vs JEV's judgment.
type TeamAnalysis struct {
	Baseline   string  `json:"baseline"`
	Jev        string  `json:"jev,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
	Eco        float64 `json:"eco,omitempty"`
	Agree      bool    `json:"agree"`
}

// RoundAnalysis is one row of the per-demo analysis.
type RoundAnalysis struct {
	Round  int           `json:"round"`
	Winner string        `json:"winner"`
	CT     *TeamAnalysis `json:"ct,omitempty"`
	T      *TeamAnalysis `json:"t,omitempty"`
}

// Analysis is the full per-demo JEV analysis, as served by
// GET /demos/:id/analysis.
type Analysis struct {
	Model       string          `json:"model,omitempty"`
	Map         string          `json:"map,omitempty"`
	Rounds      []RoundAnalysis `json:"rounds"`
	Usage       *JevUsage       `json:"usage,omitempty"`
	GeneratedAt string          `json:"generatedAt,omitempty"`
}

// analyzeDemoRounds loads a parsed demo, builds one state per round and runs
// the JEV question batch. Shared by the CLI (-jev) and the /analysis route.
// Returns the JSON-ready Analysis plus per-round detail (states) for the CLI.
func analyzeDemoRounds(demoId, key string) (*Analysis, []jevReport, error) {
	dir := demoDir(demoId)
	outData, err := os.ReadFile(filepath.Join(dir, "output.json"))
	if err != nil {
		return nil, nil, fmt.Errorf("demo %q not found (parse it first)", demoId)
	}
	var out struct {
		Map        string `json:"map"`
		RoundCount int    `json:"roundCount"`
	}
	if err := json.Unmarshal(outData, &out); err != nil {
		return nil, nil, fmt.Errorf("bad output.json: %w", err)
	}

	var rounds []slimRound
	for n := 0; n < out.RoundCount; n++ {
		data, err := os.ReadFile(filepath.Join(dir, strconv.Itoa(n)+".json"))
		if err != nil {
			break
		}
		var r slimRound
		if err := json.Unmarshal(data, &r); err != nil {
			continue
		}
		rounds = append(rounds, r)
	}
	if len(rounds) == 0 {
		return nil, nil, fmt.Errorf("no round files found for demo %q", demoId)
	}
	withEconomy := 0
	for _, r := range rounds {
		if r.Economy != nil {
			withEconomy++
		}
	}
	if withEconomy == 0 {
		return nil, nil, fmt.Errorf("no round has economy data — re-parse the demo with the current build to capture it")
	}

	dry := key == ""
	questions := economyQuestions()

	// Scores before each round, derived from previous round winners.
	scoreCT, scoreT := 0, 0
	var reports []jevReport
	analysis := &Analysis{Map: out.Map, Rounds: []RoundAnalysis{}}
	var usage JevUsage
	var model string
	jevDown := false // sticky: after a fatal JEV error (e.g. bad key), stop calling

	for i, r := range rounds {
		var recentWinners []string
		for j := 0; j < i; j++ {
			w := winnerLabel(rounds[j].Winner)
			recentWinners = append(recentWinners, w)
			if len(recentWinners) > 5 {
				recentWinners = recentWinners[1:]
			}
		}

		rep := jevReport{num: i, winner: r.Winner, econ: r.Economy}
		ra := RoundAnalysis{Round: i, Winner: winnerLabel(r.Winner)}
		if r.Economy != nil {
			ctStreak := lossStreak(rounds, i, "CT")
			tStreak := lossStreak(rounds, i, "T")
			rep.state = buildEconomyState(out.Map, i, scoreCT, scoreT, ctStreak, tStreak, recentWinners, r.Economy)
			if !dry && !jevDown {
				jev, err := jevEvaluate(key, rep.state, questions)
				if err != nil {
					rep.jevError = err
					// auth/config errors won't heal mid-run — stop calling JEV
					if strings.Contains(err.Error(), "HTTP 401") || strings.Contains(err.Error(), "HTTP 403") {
						jevDown = true
					}
				} else {
					rep.jev = jev
					model = jev.Model
					usage.InputTokens += jev.Usage.InputTokens
					usage.OutputTokens += jev.Usage.OutputTokens
				}
			}
			ra.CT = teamAnalysis(r.Economy.CT, rep, "ct")
			ra.T = teamAnalysis(r.Economy.T, rep, "t")
		}
		analysis.Rounds = append(analysis.Rounds, ra)

		// Advance the running score with this round's result.
		switch r.Winner {
		case "CT":
			scoreCT++
		case "T":
			scoreT++
		}
		reports = append(reports, rep)
	}

	analysis.Model = model
	if !dry {
		u := usage
		analysis.Usage = &u
	}
	analysis.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	return analysis, reports, nil
}

// teamAnalysis maps one team's economy + JEV answers into the JSON shape.
func teamAnalysis(t *TeamEconomy, rep jevReport, side string) *TeamAnalysis {
	if t == nil {
		return nil
	}
	ta := &TeamAnalysis{Baseline: t.Type, Agree: true}
	if rep.jev != nil {
		if ans, ok := rep.jev.Answers[side+"_buy"]; ok {
			ta.Jev = ans.Choice
			ta.Confidence = ans.Confidence
			ta.Agree = ans.Choice == t.Type
		}
		if ans, ok := rep.jev.Answers[side+"_eco"]; ok {
			ta.Eco = ans.Noul
		}
	}
	return ta
}

// AnalyzeEconomy is the -jev CLI entry point: prints the per-round baseline
// vs JEV comparison to stdout.
func AnalyzeEconomy(demoId string) error {
	analysis, reports, err := analyzeDemoRounds(demoId, os.Getenv("TYPESAFE_API_KEY"))
	if err != nil {
		return err
	}
	dry := analysis.Usage == nil
	questions := economyQuestions()

	fmt.Printf("=== round economy analysis: %s (%s, %d rounds) ===\n", demoId, analysis.Map, len(reports))
	if dry {
		fmt.Println("dry run: TYPESAFE_API_KEY not set — baseline only; set the key for live JEV analysis")
	}

	disagreements := 0
	for _, rep := range reports {
		line := fmt.Sprintf("round %2d  winner %-4s", rep.num, winnerLabel(rep.winner))
		var ctTeam, tTeam *TeamEconomy
		if rep.econ != nil {
			ctTeam = rep.econ.CT
			tTeam = rep.econ.T
		}
		for _, side := range []struct {
			label string
			team  *TeamEconomy
			buyId string
			ecoId string
		}{
			{"CT", ctTeam, "ct_buy", "ct_eco"},
			{"T", tTeam, "t_buy", "t_eco"},
		} {
			if side.team == nil {
				line += fmt.Sprintf(" | %s: --", side.label)
				continue
			}
			line += fmt.Sprintf(" | %s: %-6s", side.label, side.team.Type)
			if rep.jev != nil {
				if ans, ok := rep.jev.Answers[side.buyId]; ok {
					line += fmt.Sprintf(" jev %-6s", ans.Choice)
					if ans.Confidence < 0.5 {
						line += " (unsure)"
					}
					if ans.Choice != side.team.Type {
						line += " <== disagrees"
						disagreements++
					}
				}
				if ans, ok := rep.jev.Answers[side.ecoId]; ok {
					line += fmt.Sprintf(" eco p=%.2f", ans.Noul)
				}
			}
		}
		if rep.jevError != nil {
			line += fmt.Sprintf(" [jev error: %v]", rep.jevError)
		}
		fmt.Println(line)
	}

	if !dry {
		withJev := 0
		for _, rep := range reports {
			if rep.jev != nil {
				withJev++
			}
		}
		fmt.Printf("\nsummary: %d/%d rounds with jev data, %d baseline disagreements, %d in + %d out tokens\n",
			withJev, len(reports), disagreements, analysis.Usage.InputTokens, analysis.Usage.OutputTokens)

		if disagreements > 0 {
			fmt.Println("\n--- disagreement rounds (full states for review) ---")
			for _, rep := range reports {
				if rep.jev == nil || rep.econ == nil {
					continue
				}
				ctDisagree := rep.econ.CT != nil && rep.jev.Answers["ct_buy"].Choice != rep.econ.CT.Type
				tDisagree := rep.econ.T != nil && rep.jev.Answers["t_buy"].Choice != rep.econ.T.Type
				if !ctDisagree && !tDisagree {
					continue
				}
				fmt.Printf("\nround %d (winner %s):\n%s", rep.num, winnerLabel(rep.winner), rep.state)
			}
		}
	} else {
		// Dry run: show one example state so it's clear what would be sent.
		// Prefer a real buy round over a pistol round for the example.
		for _, rep := range reports {
			if rep.state == "" {
				continue
			}
			if rep.econ != nil && rep.econ.CT != nil && rep.econ.CT.Type == "pistol" {
				continue
			}
			fmt.Printf("\n--- example JEV request (round %d) ---\n", rep.num)
			fmt.Println("state:")
			fmt.Print(rep.state)
			q, _ := json.MarshalIndent(questions, "", "  ")
			fmt.Println("questions:")
			fmt.Println(string(q))
			break
		}
	}
	return nil
}

// analysisInflight tracks one in-flight analysis run per demo id so
// concurrent requests share a single JEV run instead of racing.
var analysisInflight sync.Map // demoId -> chan struct{}

// GetAnalysis serves GET /demos/:id/analysis. The first request runs the JEV
// analysis (a few seconds) and caches it as analysis.json in the demo's
// output dir — JEV is billed once per demo (to whoever ran it first). Later
// requests serve the cache. The key is bring-your-own: the X-TypeSafe-Key
// request header wins, TYPESAFE_API_KEY is the server-side fallback.
func GetAnalysis(c *gin.Context) {
	id := c.Param("id")
	dir := demoDir(id)
	cache := filepath.Join(dir, "analysis.json")

	if data, err := os.ReadFile(cache); err == nil {
		c.Data(200, "application/json", data)
		return
	}
	if _, err := os.Stat(filepath.Join(dir, "output.json")); err != nil {
		c.JSON(404, gin.H{"error": "demo not found"})
		return
	}
	key := strings.TrimSpace(c.GetHeader("X-TypeSafe-Key"))
	if key == "" {
		key = os.Getenv("TYPESAFE_API_KEY")
	}
	if key == "" {
		c.JSON(503, gin.H{"error": "no JEV key — add your TypeSafe key in the app settings (🔑) or set TYPESAFE_API_KEY on the server"})
		return
	}

	actual, loaded := analysisInflight.LoadOrStore(id, make(chan struct{}))
	ch := actual.(chan struct{})
	if loaded {
		// Another request is already running the analysis — wait for it,
		// then serve the cache it wrote.
		<-ch
		if data, err := os.ReadFile(cache); err == nil {
			c.Data(200, "application/json", data)
			return
		}
		c.JSON(500, gin.H{"error": "analysis failed — try again"})
		return
	}

	defer func() {
		analysisInflight.Delete(id)
		close(ch)
	}()
	analysis, _, err := analyzeDemoRounds(id, key)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if data, err := json.Marshal(analysis); err == nil {
		os.WriteFile(cache, data, 0644)
	}
	c.JSON(200, analysis)
}

func winnerLabel(w string) string {
	if w == "" {
		return "draw"
	}
	return w
}

// lossStreak counts side's consecutive losses among the rounds before roundIdx,
// looking backwards from the most recent one.
func lossStreak(rounds []slimRound, roundIdx int, side string) int {
	s := 0
	for j := roundIdx - 1; j >= 0; j-- {
		if rounds[j].Winner != side {
			s++
		} else {
			break
		}
	}
	return s
}
