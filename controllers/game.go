package controllers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"unicode"

	dem "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/msg"
	"golang.org/x/exp/slices"
)

type PlayerState struct {
	Name     string  `json:"name"`
	SteamId  string  `json:"steamId"`
	Position Vector  `json:"position"`
	Yaw      float32 `json:"yaw"`
	Team     string  `json:"team"`
	Firing   bool    `json:"firing"`
	Alive    bool    `json:"alive"`
	Blind    bool    `json:"blind"`
	Weapon   string  `json:"weapon"`
	Health   int     `json:"health"`
}

type PlayerScoreBoardState struct {
	Name    string `json:"name"`
	SteamId string `json:"steamId"`
	Kills   int    `json:"kills"`
	Deaths  int    `json:"deaths"`
	Assists int    `json:"assists"`
	Mvps    int    `json:"mvps"`
	Score   int    `json:"score"`
	Damage  int    `json:"damage"`
	Weapon  string `json:"weapon"`
}

type SmokeState struct {
	ID       int    `json:"id"`
	Position Vector `json:"position"`
}

type Grenade struct {
	Position Vector `json:"position"`
}

type FrameState struct {
	Frame        int                     `json:"frame"`
	CTScore      int                     `json:"ctScore"`
	TScore       int                     `json:"tScore"`
	Time         float64                 `json:"time"`
	PlayerStates []PlayerState           `json:"playerStates"`
	Phase        string                  `json:"phase"`
	Round        int                     `json:"round"`
	BombState    BombState               `json:"bombState"`
	Smokes       []SmokeState            `json:"smokes"`
	Flashes      []FlashState            `json:"flashes"`
	Grenades     []Grenade               `json:"grenades"`
	Hes          []HEState               `json:"hes"`
	Fires        []FireState             `json:"fires"`
	CTScoreBoard []PlayerScoreBoardState `json:"ctScoreBoard"`
	TScoreBoard  []PlayerScoreBoardState `json:"tScoreBoard"`
}

type Player struct {
	Name    *string `json:"name"`
	SteamID *string `json:"steamId"`
}
type Team struct {
	Players Player
	Side    string
}

type BombState struct {
	Planted  bool   `json:"planted"`
	Position Vector `json:"position"`
	Carrier  string `json:"carrier,omitempty"` // steam id of the player carrying the C4
}

type FireState struct {
	Position Vector `json:"position"`
}

type FlashState struct {
	ID       int    `json:"id"`
	Power    int    `json:"power"`
	Position Vector `json:"position"`
}

type HEState struct {
	ID       int    `json:"id"`
	Power    int    `json:"power"`
	Position Vector `json:"position"`
}

type Round struct {
	Round  *int         `json:"round"`
	Frames []FrameState `json:"frames"`
	Winner string       `json:"winner"`
	Kills  []KillEvent  `json:"kills"`
}

type KillEvent struct {
	Frame           int     `json:"frame"`
	Time            float64 `json:"time"`
	Attacker        string  `json:"attacker"`
	AttackerSteamId string  `json:"attackerSteamId"`
	AttackerTeam    string  `json:"attackerTeam"`
	Victim          string  `json:"victim"`
	VictimSteamId   string  `json:"victimSteamId"`
	VictimTeam      string  `json:"victimTeam"`
	Assister        string  `json:"assister,omitempty"`
	Weapon          string  `json:"weapon"`
	Headshot        bool    `json:"headshot"`
	Position        Vector  `json:"position"`
}

func teamName(t common.Team) string {
	switch t {
	case common.TeamTerrorists:
		return "T"
	case common.TeamCounterTerrorists:
		return "CT"
	}
	return ""
}

type Game struct {
	Players        []Player     `json:"players"`
	Map            string       `json:"map"`
	Frames         []FrameState `json:"frames"`
	GameStartFrame int          `json:"gameStartFrame"`
	GameEndFrame   int          `json:"gameEndFrame"`
	FrameRate      int          `json:"frameRate"`
	Rounds         []Round      `json:"rounds"`
	RoundCount     int          `json:"roundCount"`
}

type Vector struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func sortTeamScoreBoard(a, b PlayerScoreBoardState) bool {
	if a.Damage == b.Damage {
		iRunes := []rune(a.Name)
		jRunes := []rune(b.Name)

		max := len(iRunes)
		if max > len(jRunes) {
			max = len(jRunes)
		}

		for idx := 0; idx < max; idx++ {
			ir := iRunes[idx]
			jr := jRunes[idx]

			lir := unicode.ToLower(ir)
			ljr := unicode.ToLower(jr)

			if lir != ljr {
				return lir < ljr
			}

			// the lowercase runes are the same, so compare the original
			if ir != jr {
				return ir < jr
			}
		}

		// If the strings are the same up to the length of the shortest string,
		// the shorter string comes first
		return len(iRunes) < len(jRunes)
	}

	return a.Damage > b.Damage
}

// countingReader tracks how many bytes of a demo file have been consumed.
// CS2 demos don't expose a frame count until the end of the file, so parse
// progress is reported as a fraction of bytes read instead.
type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(buf []byte) (int, error) {
	n, err := c.r.Read(buf)
	c.n += int64(n)
	return n, err
}

func ParseDemo(demoId string, filePath string) Game {
	f, err := os.Open(filePath)
	if err != nil {
		log.Panic("failed to open demo file: ", err)
	}
	defer f.Close()

	fileSize := int64(0)
	if st, err := f.Stat(); err == nil {
		fileSize = st.Size()
	}
	reader := &countingReader{r: f}
	p := dem.NewParser(reader)
	defer p.Close()
	var mapName string
	var players []Player

	var gameStartFrame int
	var gameEndFrame int
	var rounds []Round
	roundKills := map[int][]KillEvent{}
	currentRound := -1
	matchStarted := false
	smokes := map[int]SmokeState{}
	flashes := map[int]FlashState{}
	hes := map[int]HEState{}
	tScore := -1
	ctScore := -1

	currentBomb := BombState{
		Planted: false,
		Position: Vector{
			X: 0,
			Y: 0,
		},
	}

	var firing []uint64
	p.RegisterEventHandler(func(e events.FrameDone) {
		if fileSize > 0 {
			progress := int(float64(reader.n) / float64(fileSize) * 100)
			updateProgress(demoId, progress, 100)
		}
		participants := p.GameState().Participants().Playing()
		var playerStates []PlayerState
		for _, element := range participants {
			team := ""
			firingNow := false
			teamId := element.TeamState.Team()
			if teamId == 2 {
				team = "T"
			}
			if teamId == 3 {
				team = "CT"
			}

			idx := slices.IndexFunc(firing, func(player uint64) bool {
				return player == element.SteamID64
			})
			if idx > -1 {
				firingNow = true
			}

			name := element.Name
			steamId := strconv.FormatUint(element.SteamID64, 10)
			alive := element.IsAlive() && element.Health() > 0
			yaw := element.ViewDirectionX()
			elementPosition := element.Position()
			blind := element.IsBlinded()

			position := Vector{
				X: elementPosition.X,
				Y: elementPosition.Y,
			}
			weapon := ""
			if element.ActiveWeapon() == nil {
				weapon = "None"
			} else {
				weapon = element.ActiveWeapon().String()
			}
			playerStates = append(playerStates,
				PlayerState{
					Name:     name,
					SteamId:  steamId,
					Position: position,
					Yaw:      yaw,
					Team:     team,
					Firing:   firingNow,
					Alive:    alive,
					Blind:    blind,
					Weapon:   weapon,
					Health:   element.Health(),
				})
		}
		phase := "PAUSED"
		firing = []uint64{}
		if p.GameState().GamePhase() == 2 {
			phase = "LIVE"
		}
		currentFrame := p.CurrentFrame()
		currentTime := p.CurrentTime().Seconds()
		round := currentRound // p.GameState().TotalRoundsPlayed()

		if !matchStarted || round < 0 {
			return
		}

		tScore = p.GameState().TeamTerrorists().Score()
		ctScore = p.GameState().TeamCounterTerrorists().Score()

		if len(rounds) <= round {
			rounds = append(rounds, Round{
				Round: &round,
				Kills: roundKills[round],
			})
		}
		frames := rounds[round].Frames

		var smokesArray []SmokeState
		for _, v := range smokes {
			smokesArray = append(smokesArray, v)
		}

		var flashArray []FlashState
		for index, v := range flashes {
			v.Power -= 1
			flashes[index] = v
			if v.Power <= 0 {
				continue
			}
			flashArray = append(flashArray, v)
		}
		var fires []FireState

		for _, v := range p.GameState().Infernos() {
			for _, fire := range v.Fires().List() {
				if fire.IsBurning {
					fires = append(fires, FireState{
						Position: Vector{
							X: fire.X,
							Y: fire.Y,
						},
					})
				}
			}
		}
		var heArray []HEState
		for _, v := range hes {
			v.Power -= 1
			hes[v.ID] = v
			if v.Power <= 0 {
				continue
			}
			heArray = append(heArray, v)
		}

		var grenades []Grenade
		for _, v := range p.GameState().GrenadeProjectiles() {
			grenades = append(grenades, Grenade{
				Position: Vector{
					X: v.Position().X,
					Y: v.Position().Y,
				},
			})
		}

		// Full bomb state: planted (event-tracked), carried (follows the
		// carrier), or dropped (last position on the ground).
		bombState := currentBomb
		if !bombState.Planted {
			if bomb := p.GameState().Bomb(); bomb != nil {
				pos := bomb.Position()
				bombState = BombState{Position: Vector{X: pos.X, Y: pos.Y}}
				if bomb.Carrier != nil {
					bombState.Carrier = strconv.FormatUint(bomb.Carrier.SteamID64, 10)
				}
			}
		}

		var ctScores []PlayerScoreBoardState
		for _, v := range p.GameState().TeamCounterTerrorists().Members() {
			weapon := ""
			if v.ActiveWeapon() == nil {
				weapon = "None"
			} else {
				weapon = v.ActiveWeapon().String()
			}
			ctScores = append(ctScores, PlayerScoreBoardState{
				Name:    v.Name,
				SteamId: strconv.FormatUint(v.SteamID64, 10),
				Kills:   v.Kills(),
				Deaths:  v.Deaths(),
				Assists: v.Assists(),
				Mvps:    v.MVPs(),
				Score:   v.Score(),
				Damage:  v.TotalDamage(),
				Weapon:  weapon,
			})
		}
		var tScores []PlayerScoreBoardState

		for _, v := range p.GameState().TeamTerrorists().Members() {
			weapon := ""
			if v.ActiveWeapon() == nil {
				weapon = "None"
			} else {
				weapon = v.ActiveWeapon().String()
			}
			tScores = append(tScores, PlayerScoreBoardState{
				Name:    v.Name,
				SteamId: strconv.FormatUint(v.SteamID64, 10),
				Kills:   v.Kills(),
				Deaths:  v.Deaths(),
				Assists: v.Assists(),
				Mvps:    v.MVPs(),
				Score:   v.Score(),
				Damage:  v.TotalDamage(),
				Weapon:  weapon,
			})
		}
		sort.Slice(ctScores, func(i, j int) bool {
			return sortTeamScoreBoard(ctScores[i], ctScores[j])
		})
		sort.Slice(tScores, func(i, j int) bool {
			return sortTeamScoreBoard(tScores[i], tScores[j])
		})

		frames = append(frames, FrameState{
			Frame:        currentFrame,
			Time:         currentTime,
			PlayerStates: playerStates,
			Phase:        phase,
			Round:        round,
			BombState:    bombState,
			Smokes:       smokesArray,
			Flashes:      flashArray,
			Hes:          heArray,
			CTScore:      ctScore,
			TScore:       tScore,
			Grenades:     grenades,
			Fires:        fires,
			CTScoreBoard: ctScores,
			TScoreBoard:  tScores,
		})
		rounds[round].Frames = frames
	})
	p.RegisterEventHandler(func(e events.AnnouncementMatchStarted) {
		gameStartFrame = p.CurrentFrame()
		// fmt.Println(p.GameState().TeamTerrorists().Members())
	})
	p.RegisterEventHandler(func(e events.MatchStart) {
		gameStartFrame = p.CurrentFrame()
		// Discard the knife round so the first real round overwrites it.
		rounds = []Round{}
		roundKills = map[int][]KillEvent{}
		currentRound = -1
		matchStarted = true
	})
	p.RegisterEventHandler(func(e events.AnnouncementWinPanelMatch) {
		gameEndFrame = p.CurrentFrame()
	})
	p.RegisterEventHandler(func(e events.PlayerConnect) {
		steamId := strconv.FormatUint(e.Player.SteamID64, 10)
		players = append(players, Player{
			Name:    &e.Player.Name,
			SteamID: &steamId,
		})
	})

	p.RegisterEventHandler(func(e events.SmokeStart) {
		if e.Grenade != nil && e.Grenade.Entity != nil {
			// fmt.Printf("Smoke start %d \n", e.Grenade.Entity.ID())

			smokes[e.Grenade.Entity.ID()] = SmokeState{
				ID: e.Grenade.Entity.ID(),
				Position: Vector{
					X: e.Position.X,
					Y: e.Position.Y,
				},
			}
		}
	})
	p.RegisterEventHandler(func(e events.SmokeExpired) {
		if e.Grenade != nil && e.Grenade.Entity != nil {
			delete(smokes, e.Grenade.Entity.ID())
			// fmt.Println(len(smokes))
		}
	})

	p.RegisterEventHandler(func(e events.HeExplode) {
		if e.Grenade != nil && e.Grenade.Entity != nil {
			hes[e.Grenade.Entity.ID()] = HEState{
				ID:    e.Grenade.Entity.ID(),
				Power: 10,
				Position: Vector{
					X: e.Position.X,
					Y: e.Position.Y,
				},
			}
		}
	})

	p.RegisterEventHandler(func(e events.FlashExplode) {
		if e.Grenade != nil && e.Grenade.Entity != nil {
			flashes[e.Grenade.Entity.ID()] = FlashState{
				ID:    e.Grenade.Entity.ID(),
				Power: 100,
				Position: Vector{
					X: e.Position.X,
					Y: e.Position.Y,
				},
			}
		}
	})
	p.RegisterEventHandler(func(e events.RoundStart) {
		// fmt.Println("New round ------------------------------------------------------ ")
		if matchStarted {
			currentRound++
		}
		// CS2 lets players plant the bomb (and throw nades) after RoundEnd,
		// during the round-over period. Reset all transient state here so a
		// phantom post-round plant doesn't leak into the next round's frames.
		smokes = map[int]SmokeState{}
		flashes = map[int]FlashState{}
		hes = map[int]HEState{}
		currentBomb = BombState{
			Planted: false,
			Position: Vector{
				X: 0,
				Y: 0,
			},
		}
	})
	p.RegisterEventHandler(func(e events.RoundEnd) {
		// fmt.Println("Round ended ------------------------------------------------------ ")

		winner := ""
		if e.Winner == common.TeamTerrorists {
			winner = "T"
		} else if e.Winner == common.TeamCounterTerrorists {
			winner = "CT"
		}
		if currentRound >= 0 && currentRound < len(rounds) {
			rounds[currentRound].Winner = winner
		}

		smokes = map[int]SmokeState{}
		flashes = map[int]FlashState{}
		hes = map[int]HEState{}
		currentBomb = BombState{
			Planted: false,
			Position: Vector{
				X: 0,
				Y: 0,
			},
		}
	})

	p.RegisterEventHandler(func(e events.WeaponFire) {
		// fmt.Println(e.Shooter)
		if e.Shooter != nil {
			firing = append(firing, e.Shooter.SteamID64)
		}
	})
	p.RegisterEventHandler(func(e events.Kill) {
		if !matchStarted || currentRound < 0 {
			return
		}
		k := KillEvent{
			Frame:    p.CurrentFrame(),
			Time:     p.CurrentTime().Seconds(),
			Headshot: e.IsHeadshot,
		}
		if e.Weapon != nil {
			k.Weapon = e.Weapon.String()
		}
		if e.Killer != nil {
			k.Attacker = e.Killer.Name
			k.AttackerSteamId = strconv.FormatUint(e.Killer.SteamID64, 10)
			k.AttackerTeam = teamName(e.Killer.TeamState.Team())
		}
		if e.Victim != nil {
			k.Victim = e.Victim.Name
			k.VictimSteamId = strconv.FormatUint(e.Victim.SteamID64, 10)
			k.VictimTeam = teamName(e.Victim.TeamState.Team())
			pos := e.Victim.Position()
			k.Position = Vector{X: pos.X, Y: pos.Y}
		}
		if e.Assister != nil && e.Assister.Name != "" {
			k.Assister = e.Assister.Name
		}
		if currentRound < len(rounds) {
			rounds[currentRound].Kills = append(rounds[currentRound].Kills, k)
		} else {
			roundKills[currentRound] = append(roundKills[currentRound], k)
		}
	})
	p.RegisterEventHandler(func(e events.BombPlanted) {
		pos := e.BombEvent.Player.Position()
		currentBomb = BombState{
			Planted: true,
			Position: Vector{
				X: pos.X,
				Y: pos.Y,
			},
		}
	})

	// p.RegisterEventHandler(func(e events.Kill) {
	// 	var hs string
	// 	if e.IsHeadshot {
	// 		hs = " (HS)"
	// 	}
	// 	var wallBang string
	// 	if e.PenetratedObjects > 0 {
	// 		wallBang = " (WB)"
	// 	}
	// 	// fmt.Printf("%s <%v%s%s> %s\n", e.Killer, e.Weapon, hs, wallBang, e.Victim)
	// 	fmt.Printf("%s <%v%s%s> %s\n", nil, e.Weapon, hs, wallBang, nil)
	// })

	p.RegisterNetMessageHandler(func(m *msg.CSVCMsg_ServerInfo) {
		mapName = m.GetMapName()
		fmt.Println(mapName)
	})

	// Parse to end
	if err = p.ParseToEnd(); err != nil {
		log.Println("parse ended with error:", err)
	}
	frameRate := 60 // int(p.Header().PlaybackFrames / int(p.Header().PlaybackTime.Seconds()))
	game := Game{
		Players:        players,
		Map:            mapName,
		Rounds:         rounds,
		GameStartFrame: gameStartFrame,
		GameEndFrame:   gameEndFrame,
		FrameRate:      frameRate,
		RoundCount:     len(rounds),
	}
	fmt.Println("Parsed game")
	outDir := filepath.Join(outputDir, demoId)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		log.Panic("failed to create output dir: ", err)
	}
	for i, round := range game.Rounds {
		fmt.Println("round", i+1, "/", len(game.Rounds))
		roundJson, err := json.Marshal(round)
		if err != nil {
			log.Panic("failed to marshal round: ", err)
		}
		roundFile := filepath.Join(outDir, strconv.Itoa(*round.Round)+".json")
		if err := os.WriteFile(roundFile, roundJson, 0644); err != nil {
			log.Panic("failed to write round file: ", err)
		}
	}

	game.Rounds = nil
	jsonObj, err := json.Marshal(game)
	if err != nil {
		log.Panic("failed to marshal game: ", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "output.json"), jsonObj, 0644); err != nil {
		log.Panic("failed to write output file: ", err)
	}
	return game
}
