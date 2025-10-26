package controllers

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	dem "github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/events"
	"github.com/markus-wa/demoinfocs-golang/v5/pkg/demoinfocs/msg"
	"golang.org/x/exp/slices"
)

type PlayerState struct {
	Name     string  `json:"name"`
	SteamId  string  `json:"steamId"`
	Kills    int     `json:"kills"`
	Deaths   int     `json:"deaths"`
	Assists  int     `json:"assists"`
	Mvps     int     `json:"mvps"`
	Position Vector  `json:"position"`
	Yaw      float32 `json:"yaw"`
	Team     string  `json:"team"`
	Firing   bool    `json:"firing"`
	Alive    bool    `json:"alive"`
}

type SmokeState struct {
	ID       int    `json:"id"`
	Position Vector `json:"position"`
}

type FrameState struct {
	Frame        int           `json:"frame"`
	CTScore      int           `json:"ctScore"`
	TScore       int           `json:"tScore"`
	Time         float64       `json:"time"`
	PlayerStates []PlayerState `json:"playerStates"`
	Phase        string        `json:"phase"`
	Round        int           `json:"round"`
	BombState    BombState     `json:"bombState"`
	Smokes       []SmokeState  `json:"smokes"`
	Flashes      []FlashState  `json:"flashes"`
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

type FlashState struct {
	ID       int    `json:"id"`
	Power    int    `json:"power"`
	Position Vector `json:"position"`
}

type Round struct {
	Round  *int         `json:"round"`
	Frames []FrameState `json:"frames"`
}

type Game struct {
	Players        []Player     `json:"players"`
	Map            string       `json:"map"`
	Frames         []FrameState `json:"frames"`
	GameStartFrame int          `json:"gameStartFrame"`
	FrameRate      int          `json:"frameRate"`
	Rounds         []Round      `json:"rounds"`
}

type Vector struct {
	X *float64 `json:"x"`
	Y *float64 `json:"y"`
}

func GetGame(start int, limit int) Game {
	f, err := os.Open("./test.dem")
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
	var currentRound = 0
	smokes := map[int]SmokeState{}
	flashes := map[int]FlashState{}
	tScore := -1
	ctScore := -1

	var currentBomb = BombState{
		Planted: false,
		Position: Vector{
			X: nil,
			Y: nil,
		},
	}

	var firing []uint64
	p.RegisterEventHandler(func(e events.FrameDone) {

		if p.CurrentFrame()%1 == 0 {
			// TODO get state for all players at any frame
			var participants = p.GameState().Participants().Playing()
			var playerStates []PlayerState
			for _, element := range participants {
				var team = ""
				var firingNow = false
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

				var name = element.Name
				var kills = element.Kills()
				var deaths = element.Deaths()
				var assists = element.Assists()
				var mvps = element.MVPs()
				var steamId = strconv.FormatUint(element.SteamID64, 10)
				var alive = element.IsAlive() && element.Health() > 0
				var yaw = element.ViewDirectionX()
				var elementPosition = element.Position()

				element.ViewDirectionX()

				var position = Vector{
					X: &elementPosition.X,
					Y: &elementPosition.Y,
				}

				playerStates = append(playerStates,
					PlayerState{
						Name:     name,
						Kills:    kills,
						Deaths:   deaths,
						Assists:  assists,
						Mvps:     mvps,
						SteamId:  steamId,
						Position: position,
						Yaw:      yaw,
						Team:     team,
						Firing:   firingNow,
						Alive:    alive,
					})
			}
			var phase = "PAUSED"
			firing = []uint64{}
			if p.GameState().GamePhase() == 2 {
				phase = "LIVE"
			}
			var currentFrame = p.CurrentFrame()
			var currentTime = p.CurrentTime().Seconds()
			var round = currentRound // p.GameState().TotalRoundsPlayed()

			tScore = p.GameState().TeamTerrorists().Score()
			ctScore = p.GameState().TeamCounterTerrorists().Score()

			if len(rounds) <= round {
				rounds = append(rounds, Round{
					Round: &round,
				})
			}
			var frames = rounds[round].Frames

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

			frames = append(frames, FrameState{
				Frame:        currentFrame,
				Time:         currentTime,
				PlayerStates: playerStates,
				Phase:        phase,
				Round:        round,
				BombState:    currentBomb,
				Smokes:       smokesArray,
				Flashes:      flashArray,
				CTScore:      ctScore,
				TScore:       tScore,
			})
			rounds[round].Frames = frames

		}
	})
	p.RegisterEventHandler(func(e events.AnnouncementMatchStarted) {
		gameStartFrame = p.CurrentFrame()
		fmt.Println(p.GameState().TeamTerrorists().Members())

	})
	p.RegisterEventHandler(func(e events.PlayerConnect) {
		var steamId = strconv.FormatUint(e.Player.SteamID64, 10)
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
					X: &e.Position.X,
					Y: &e.Position.Y,
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
	p.RegisterEventHandler(func(e events.FlashExplode) {
		fmt.Println("Flash popped! ")
		if e.Grenade != nil && e.Grenade.Entity != nil {

			flashes[e.Grenade.Entity.ID()] = FlashState{
				ID:    e.Grenade.Entity.ID(),
				Power: 100,
				Position: Vector{
					X: &e.Position.X,
					Y: &e.Position.Y,
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

		smokes = map[int]SmokeState{}
		flashes = map[int]FlashState{}
		currentBomb = BombState{
			Planted: false,
			Position: Vector{
				X: nil,
				Y: nil,
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
		var pos = e.BombEvent.Player.Position()
		currentBomb = BombState{
			Planted: true,
			Position: Vector{
				X: &pos.X,
				Y: &pos.Y,
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
	var frameRate int = 60 //int(p.Header().PlaybackFrames / int(p.Header().PlaybackTime.Seconds()))
	var game = Game{
		Players: players,
		Map:     mapName, //p.Header().MapName,
		// Frames:  frames[start:lastFrame],
		Rounds:         rounds,
		GameStartFrame: gameStartFrame,
		FrameRate:      frameRate,
	}
	var gameModel = Game{
		Map:       game.Map,
		FrameRate: game.FrameRate,
		Rounds:    game.Rounds,
		Players:   game.Players,
	}
	fmt.Println("Parsed game")
	for _, round := range game.Rounds {
		roundJson, err := json.Marshal(round)
		err = os.WriteFile("./server/data/output/"+strconv.Itoa(*round.Round)+".json", roundJson, 0644)
		if err != nil {
			log.Panic("Something went wrong: ", err)
		}
	}

	game.Rounds = nil
	jsonObj, err := json.Marshal(game)

	err = os.WriteFile("./server/data/output/output.json", jsonObj, 0644)
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
