package controllers

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
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
}

type Game struct {
	Players        []Player     `json:"players"`
	Map            string       `json:"map"`
	Frames         []FrameState `json:"frames"`
	GameStartFrame int          `json:"gameStartFrame"`
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

func GetGame(start int, limit int) Game {
	return ParseDemo("local", "./test.dem")
}

func ParseDemo(demoId string, filePath string) Game {
	f, err := os.Open(filePath)
	if err != nil {
		log.Panic("failed to open demo file: ", err)
	}
	defer f.Close()

	p := dem.NewParser(f)
	defer p.Close()
	var mapName string
	// var frames []*model.FrameState
	var players []Player

	var gameStartFrame int
	var rounds []Round
	currentRound := 0
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
		if p.CurrentFrame()%1 == 0 {
			// TODO get state for all players at any frame
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

				element.ViewDirectionX()

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

			tScore = p.GameState().TeamTerrorists().Score()
			ctScore = p.GameState().TeamCounterTerrorists().Score()

			if len(rounds) <= round {
				rounds = append(rounds, Round{
					Round: &round,
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
				BombState:    currentBomb,
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

		}
	})
	p.RegisterEventHandler(func(e events.AnnouncementMatchStarted) {
		gameStartFrame = p.CurrentFrame()
		// fmt.Println(p.GameState().TeamTerrorists().Members())
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
		currentRound = p.GameState().TotalRoundsPlayed() + 1
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
	p.RegisterEventHandler(func(e events.BombPlanted) {
		// fmt.Printf("%s Planted the bomb \n", e.BombEvent.Player.Position())
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
	err = p.ParseToEnd()
	fmt.Println(err)
	fmt.Println(mapName)
	frameRate := 60 // int(p.Header().PlaybackFrames / int(p.Header().PlaybackTime.Seconds()))
	game := Game{
		Players: players,
		Map:     mapName, // p.Header().MapName,
		// Frames:  frames[start:lastFrame],
		Rounds:         rounds,
		GameStartFrame: gameStartFrame,
		FrameRate:      frameRate,
		RoundCount:     len(rounds),
	}
	gameModel := Game{
		Map:        game.Map,
		FrameRate:  game.FrameRate,
		Rounds:     game.Rounds,
		Players:    game.Players,
		RoundCount: game.RoundCount,
	}
	fmt.Println("Parsed game")
	outDir := "./controllers/data/output/" + demoId
	os.MkdirAll(outDir, 0755)
	for i, round := range game.Rounds {
		fmt.Println(i, '/', len(game.Rounds))
		roundJson, err := json.Marshal(round)
		err = os.WriteFile(outDir+"/"+strconv.Itoa(*round.Round)+".json", roundJson, 0644)
		if err != nil {
			log.Panic("Something went wrong: ", err)
		}
	}

	game.Rounds = nil
	jsonObj, err := json.Marshal(game)

	err = os.WriteFile(outDir+"/output.json", jsonObj, 0644)
	if err != nil {
		log.Panic("failed to parse demo: ", err)
	}
	return gameModel
}

func Min(x, y int) int {
	if x > y {
		return y
	}
	return x
}

func Max(x, y int) int {
	if x < y {
		return y
	}
	return x
}
