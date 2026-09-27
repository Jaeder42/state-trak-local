package controllers

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// Post-plant positioning analysis.
//
// Deterministic (no LLM, no key): computed from the round JSON files that
// ParseDemo() already wrote, and cached as postplant.json in the demo's
// output dir so the (moderately expensive) scan runs once per demo.
//
// For every round with a bomb plant it captures where both teams stood after
// the plant and how the round resolved, so the client can correlate
// positioning with outcome:
//   - T setup: where the alive Ts were 5s after the plant, how spread out
//     they were, how far from the bomb, and how much they moved between
//     +5s and +15s (holding a position vs pushing/peeking)
//   - CT retake: when the first CT reached the bomb, and how bunched the
//     CTs entering the site were (grouped retake vs staggered)
//
// Phantom plants (CS2 lets players plant after RoundEnd, during the
// round-over period — see AGENTS.md) are detected and skipped: the
// RoundEnd handler resets bombState, so a real plant's planted=true run is
// always followed by planted=false frames within the same round, while a
// phantom plant's run extends to the round's last frame. This is
// deliberately phase-independent — the LIVE phase heuristic in game.go never
// fires in the second half of CS2 demos (GamePhase==2 is only set once per
// half), so phase can't be trusted here.

const (
	ppSetupDelay  = 5.0   // seconds after the plant to sample setup positions
	ppMoveDelay   = 15.0  // movement is measured from setup (+5s) to here
	ppEntryRadius = 500.0 // world units: a CT within this distance of the bomb counts as "reached the site"
)

// PostPlantPlayer is one player's position at a post-plant sample moment.
type PostPlantPlayer struct {
	SteamId    string  `json:"steamId"`
	Name       string  `json:"name"`
	Position   Vector  `json:"position"`
	DistToBomb float64 `json:"distToBomb"`
	Alive      bool    `json:"alive"` // alive at the sample moment (false = last position before dying)
}

// PostPlantRound is the post-plant story of one planted round.
type PostPlantRound struct {
	Round           int               `json:"round"`
	Winner          string            `json:"winner"`
	Outcome         string            `json:"outcome"`         // defused | exploded | eliminated
	PlantAt         float64           `json:"plantAt"`         // seconds into the round (incl. freeze time)
	PlantFrameIndex int               `json:"plantFrameIndex"` // index into the round's frames array (jump target)
	BombPos         Vector            `json:"bombPos"`
	TAliveAtPlant   int               `json:"tAliveAtPlant"`
	CTAliveAtPlant  int               `json:"ctAliveAtPlant"`
	TSetup          []PostPlantPlayer `json:"tSetup"`            // Ts alive at plant+5s — where they set up
	CTSetup         []PostPlantPlayer `json:"ctSetup"`           // CTs alive at plant+5s — retake staging
	CTEntry         []PostPlantPlayer `json:"ctEntry,omitempty"` // CTs at the bomb when the retake began
	TSpread         *float64          `json:"tSpread"`           // avg pairwise distance between T setups
	TAvgBombDist    *float64          `json:"tAvgBombDist"`      // avg distance from T setups to the bomb
	TMovement       *float64          `json:"tMovement"`         // avg distance moved between +5s and +15s
	CTEntryTime     *float64          `json:"ctEntryTime"`       // s from plant until a CT reached the bomb; null = nobody came
	CTEntrySpread   *float64          `json:"ctEntrySpread"`     // avg pairwise distance between entering CTs
	PostDeathsT     int               `json:"postDeathsT"`       // T players who died after the plant
	PostDeathsCT    int               `json:"postDeathsCT"`      // CT players who died after the plant
}

// PostPlantSummary counts the planted rounds by resolution.
type PostPlantSummary struct {
	Planted    int `json:"planted"`
	Defused    int `json:"defused"`
	Exploded   int `json:"exploded"`
	Eliminated int `json:"eliminated"` // Ts won by killing every CT after the plant
	NoRetake   int `json:"noRetake"`   // T wins where no CT ever reached the bomb
}

type PostPlantAnalysis struct {
	Rounds  []PostPlantRound `json:"rounds"`
	Summary PostPlantSummary `json:"summary"`
}

// Minimal round-file projection — only the fields the analysis needs.
type ppFrame struct {
	Time         float64     `json:"time"`
	PlayerStates []ppPlayer  `json:"playerStates"`
	BombState    ppBombState `json:"bombState"`
}

type ppPlayer struct {
	Name     string `json:"name"`
	SteamId  string `json:"steamId"`
	Team     string `json:"team"`
	Alive    bool   `json:"alive"`
	Position Vector `json:"position"`
}

type ppBombState struct {
	Planted  bool   `json:"planted"`
	Position Vector `json:"position"`
}

type ppKill struct {
	Time       float64 `json:"time"`
	VictimTeam string  `json:"victimTeam"`
}

type ppRoundFile struct {
	Round  *int      `json:"round"`
	Winner string    `json:"winner"`
	Frames []ppFrame `json:"frames"`
	Kills  []ppKill  `json:"kills"`
}

// plantedTrueProbe skips round files without any plant before the (much more
// expensive) full unmarshal. It matches the marshalled BombState output.
var plantedTrueProbe = []byte(`"planted":true`)

func ppDist(a, b Vector) float64 {
	return math.Hypot(a.X-b.X, a.Y-b.Y)
}

func ppAvgPairwise(pts []Vector) *float64 {
	if len(pts) < 2 {
		return nil
	}
	total := 0.0
	pairs := 0
	for i := range pts {
		for j := i + 1; j < len(pts); j++ {
			total += ppDist(pts[i], pts[j])
			pairs++
		}
	}
	avg := total / float64(pairs)
	return &avg
}

func ppAvgFloats(vals []float64) *float64 {
	if len(vals) == 0 {
		return nil
	}
	total := 0.0
	for _, v := range vals {
		total += v
	}
	avg := total / float64(len(vals))
	return &avg
}

func ppCountAlive(frame ppFrame, team string) int {
	n := 0
	for _, p := range frame.PlayerStates {
		if p.Team == team && p.Alive {
			n++
		}
	}
	return n
}

// ppSetupPlayers returns the players of `team` that were alive at the
// plant, with their position at the setup sample — or, if they died before
// it, their last position while alive (where they fell defending). Metrics
// (spread, distance) use only the alive ones; the client marks the rest.
func ppSetupPlayers(frames []ppFrame, plantIdx, setupIdx int, team string, bomb Vector) []PostPlantPlayer {
	var out []PostPlantPlayer
	lastPos := map[string]Vector{}
	for i := plantIdx; i <= setupIdx; i++ {
		for _, p := range frames[i].PlayerStates {
			if p.Team == team && p.Alive {
				lastPos[p.SteamId] = p.Position
			}
		}
	}
	aliveAtSetup := map[string]bool{}
	for _, p := range frames[setupIdx].PlayerStates {
		if p.Team == team && p.Alive {
			aliveAtSetup[p.SteamId] = true
		}
	}
	// stable order: the plant frame's player order
	for _, p := range frames[plantIdx].PlayerStates {
		if p.Team != team || !p.Alive {
			continue
		}
		pos, ok := lastPos[p.SteamId]
		if !ok {
			continue // dropped out of the demo entirely
		}
		out = append(out, PostPlantPlayer{
			SteamId:    p.SteamId,
			Name:       p.Name,
			Position:   pos,
			DistToBomb: ppDist(pos, bomb),
			Alive:      aliveAtSetup[p.SteamId],
		})
	}
	return out
}

// ppFrameAtOrBefore returns the latest frame index in [from, to] whose time
// is <= t (frames are recorded in demo order, time monotonic).
func ppFrameAtOrBefore(frames []ppFrame, from, to int, t float64) int {
	idx := from
	for i := from; i <= to; i++ {
		if frames[i].Time <= t {
			idx = i
		} else {
			break
		}
	}
	return idx
}

// ppAnalyzeRound extracts the post-plant story of one round. ok=false means
// the round has no usable plant (none, or a phantom planted after RoundEnd).
func ppAnalyzeRound(rf ppRoundFile, roundNum int) (*PostPlantRound, bool) {
	frames := rf.Frames
	if len(frames) < 2 {
		return nil, false
	}

	// First planted=true run: [P, E). E is the first planted=false frame
	// after it — the RoundEnd reset. Live play for this round ends at E-1.
	P := -1
	for i := range frames {
		if frames[i].BombState.Planted {
			P = i
			break
		}
	}
	if P < 0 {
		return nil, false
	}
	E := -1
	for i := P + 1; i < len(frames); i++ {
		if !frames[i].BombState.Planted {
			E = i
			break
		}
	}
	// No planted=false frame after the run: the bomb was planted after
	// RoundEnd (phantom plant during the round-over period) — not a real
	// post-plant round.
	if E < 0 {
		return nil, false
	}

	plantFrame := frames[P]
	plantTime := plantFrame.Time
	bombPos := plantFrame.BombState.Position
	lastLive := E - 1 // last frame of live play (before the RoundEnd reset)

	pr := PostPlantRound{
		Round:           roundNum,
		Winner:          rf.Winner,
		PlantAt:         plantTime - frames[0].Time,
		PlantFrameIndex: P,
		BombPos:         bombPos,
		TAliveAtPlant:   ppCountAlive(plantFrame, "T"),
		CTAliveAtPlant:  ppCountAlive(plantFrame, "CT"),
	}

	// Outcome: with a real plant, CT can only win by defusing. A T win is an
	// explosion unless every CT was already dead when the round ended.
	switch {
	case rf.Winner == "CT":
		pr.Outcome = "defused"
	case rf.Winner == "T":
		if ppCountAlive(frames[lastLive], "CT") == 0 {
			pr.Outcome = "eliminated"
		} else {
			pr.Outcome = "exploded"
		}
	default:
		return nil, false // no winner recorded — can't correlate with outcome
	}

	// Setup sample: plant+5s, clamped to live play. Players alive at the
	// plant that died before the sample keep their last position (alive=false).
	setupIdx := ppFrameAtOrBefore(frames, P, lastLive, plantTime+ppSetupDelay)
	pr.TSetup = ppSetupPlayers(frames, P, setupIdx, "T", bombPos)
	pr.CTSetup = ppSetupPlayers(frames, P, setupIdx, "CT", bombPos)

	var setupPts []Vector
	var bombDists []float64
	for _, p := range pr.TSetup {
		if !p.Alive {
			continue // metrics describe the living setup only
		}
		setupPts = append(setupPts, p.Position)
		bombDists = append(bombDists, p.DistToBomb)
	}
	pr.TSpread = ppAvgPairwise(setupPts)
	pr.TAvgBombDist = ppAvgFloats(bombDists)

	// Movement: T players alive at both samples, plant+5s → plant+15s
	// (clamped to live play). Holding a position vs pushing/peeking.
	moveIdx := ppFrameAtOrBefore(frames, setupIdx, lastLive, plantTime+ppMoveDelay)
	if moveIdx > setupIdx {
		later := map[string]Vector{}
		for _, p := range frames[moveIdx].PlayerStates {
			if p.Alive {
				later[p.SteamId] = p.Position
			}
		}
		var moves []float64
		for _, p := range pr.TSetup {
			if !p.Alive {
				continue
			}
			if pos, ok := later[p.SteamId]; ok {
				moves = append(moves, ppDist(p.Position, pos))
			}
		}
		pr.TMovement = ppAvgFloats(moves)
	}

	// CT retake entry: first live-play frame with an alive CT close to the
	// bomb. CTs already inside the radius at the plant count as immediate.
	entryIdx := -1
	for i := P; i <= lastLive; i++ {
		for _, p := range frames[i].PlayerStates {
			if p.Team == "CT" && p.Alive && ppDist(p.Position, bombPos) <= ppEntryRadius {
				entryIdx = i
				break
			}
		}
		if entryIdx >= 0 {
			break
		}
	}
	if entryIdx >= 0 {
		t := frames[entryIdx].Time - plantTime
		pr.CTEntryTime = &t
		for _, p := range frames[entryIdx].PlayerStates {
			if p.Team == "CT" && p.Alive && ppDist(p.Position, bombPos) <= ppEntryRadius {
				pr.CTEntry = append(pr.CTEntry, PostPlantPlayer{
					SteamId:    p.SteamId,
					Name:       p.Name,
					Position:   p.Position,
					DistToBomb: ppDist(p.Position, bombPos),
					Alive:      true,
				})
			}
		}
		var entryPts []Vector
		for _, p := range pr.CTEntry {
			entryPts = append(entryPts, p.Position)
		}
		pr.CTEntrySpread = ppAvgPairwise(entryPts)
	}

	// Deaths after the plant (kills carry absolute demo time, like frames).
	for _, k := range rf.Kills {
		if k.Time < plantTime {
			continue
		}
		switch k.VictimTeam {
		case "T":
			pr.PostDeathsT++
		case "CT":
			pr.PostDeathsCT++
		}
	}

	return &pr, true
}

func ppSummarize(rounds []PostPlantRound) PostPlantSummary {
	s := PostPlantSummary{Planted: len(rounds)}
	for _, r := range rounds {
		switch r.Outcome {
		case "defused":
			s.Defused++
		case "exploded":
			s.Exploded++
		case "eliminated":
			s.Eliminated++
		}
		if r.CTEntryTime == nil && r.Winner == "T" {
			s.NoRetake++
		}
	}
	return s
}

// computePostPlant scans a demo's round files for planted rounds and builds
// the post-plant analysis.
func computePostPlant(dir string) (*PostPlantAnalysis, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	// numeric .json round files, sorted by round number (like GetRounds)
	type roundFile struct {
		num  int
		path string
	}
	var files []roundFile
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		num, err := strconv.Atoi(strings.TrimSuffix(name, ".json"))
		if err != nil {
			continue // output.json, meta.json, analysis.json, postplant.json…
		}
		files = append(files, roundFile{num: num, path: filepath.Join(dir, name)})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].num < files[j].num })

	var rounds []PostPlantRound
	for _, f := range files {
		data, err := os.ReadFile(f.path)
		if err != nil {
			continue
		}
		if !bytes.Contains(data, plantedTrueProbe) {
			continue // no plant in this round — skip the unmarshal
		}
		var rf ppRoundFile
		if err := json.Unmarshal(data, &rf); err != nil {
			continue
		}
		roundNum := f.num
		if rf.Round != nil {
			roundNum = *rf.Round
		}
		if pr, ok := ppAnalyzeRound(rf, roundNum); ok {
			rounds = append(rounds, *pr)
		}
	}

	return &PostPlantAnalysis{Rounds: rounds, Summary: ppSummarize(rounds)}, nil
}

// GetPostPlant serves the post-plant positioning analysis, computing it on
// first request and caching it as postplant.json in the demo's output dir.
func GetPostPlant(c *gin.Context) {
	demoId := c.Param("id")
	dir := demoDir(demoId)

	cacheFile := filepath.Join(dir, "postplant.json")
	if data, err := os.ReadFile(cacheFile); err == nil {
		c.Data(200, "application/json", data)
		return
	}

	analysis, err := computePostPlant(dir)
	if err != nil {
		c.JSON(404, gin.H{"error": "demo not found"})
		return
	}

	data, err := json.Marshal(analysis)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	// Best-effort cache — serving without it is fine too.
	_ = os.WriteFile(cacheFile, data, 0644)
	c.Data(200, "application/json", data)
}
