package controllers

import (
	"log"
	"os"
	"strconv"

	"github.com/golang/geo/r3"
	dem "github.com/markus-wa/demoinfocs-golang/v3/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v3/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v3/pkg/demoinfocs/events"
	"golang.org/x/exp/slices"
	"jaeder42.tech/state-trak-local/graph/model"
)

type PlayerState struct {
	Name        string    `json:"name"`
	SteamId     uint64    `json:"steamId"`
	Kills       int       `json:"kills"`
	Deaths      int       `json:"deaths"`
	Assists     int       `json:"assists"`
	Mvps        int       `json:"mvps"`
	Position    r3.Vector `json:"position"`
	EyePosition r3.Vector `json:"eyePosition"`
	Team        string    `json:"team"`
	Firing      bool      `json:"firing"`
	Alive       bool      `json:"alive"`
}

type FrameState struct {
	Frame        int           `json:"frame"`
	Time         float64       `json:"time"`
	PlayerStates []PlayerState `json:"playerState"`
}

type Game struct {
	Players        []*model.Player     `json:"players"`
	Map            string              `json:"map"`
	Frames         []*model.FrameState `json:"frames"`
	GameStartFrame int                 `json:"gameStartFrame"`
	FrameRate      int                 `json:"frameRate"`
}

func GetGame() model.Game {
	f, err := os.Open("./test.dem")
	if err != nil {
		log.Panic("failed to open demo file: ", err)
	}
	defer f.Close()

	p := dem.NewParser(f)
	defer p.Close()
	var frames []*model.FrameState
	var players []*model.Player
	var gameStartFrame int

	var firing []uint64
	p.RegisterEventHandler(func(e events.FrameDone) {

		// TODO get state for all players at any frame
		var participants = p.GameState().Participants().Playing()
		var playerStates []*model.PlayerState
		for _, element := range participants {
			var team = "T"
			var firingNow = false
			idx := slices.IndexFunc(p.GameState().TeamCounterTerrorists().Members(), func(c *common.Player) bool {
				return c.SteamID64 == element.SteamID64
			})
			if idx > -1 {
				team = "CT"
			}
			idx = slices.IndexFunc(firing, func(player uint64) bool {
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

			var elementPosition = element.Position()
			var position = model.Vector{
				X: &elementPosition.X,
				Y: &elementPosition.Y,
			}

			playerStates = append(playerStates,
				&model.PlayerState{
					Name:     &name,
					Kills:    &kills,
					Deaths:   &deaths,
					Assists:  &assists,
					Mvps:     &mvps,
					SteamID:  &steamId,
					Position: &position,
					// EyePosition: element.PositionEyes(),
					Team:   &team,
					Firing: &firingNow,
					Alive:  &alive,
				})
		}
		var currentFrame = p.CurrentFrame()
		var currentTime = p.CurrentTime().Seconds()
		frames = append(frames, &model.FrameState{
			Frame:        &currentFrame,
			Time:         &currentTime,
			PlayerStates: playerStates,
		})
		firing = nil

	})
	p.RegisterEventHandler(func(e events.AnnouncementMatchStarted) {
		gameStartFrame = p.CurrentFrame()
	})
	p.RegisterEventHandler(func(e events.PlayerConnect) {
		var steamId = strconv.FormatUint(e.Player.SteamID64, 10)
		players = append(players, &model.Player{
			Name:    &e.Player.Name,
			SteamID: &steamId,
		})
	})
	// p.RegisterEventHandler(func(e events.MatchStart) {
	// 	fmt.Println("Game started --------------------------------------------------")
	// })

	// p.RegisterEventHandler(func(e events.RoundStart) {
	// 	fmt.Println("New round ------------------------------------------------------ ")
	// })
	// p.RegisterEventHandler(func(e events.RoundEnd) {
	p.RegisterEventHandler(func(e events.WeaponFire) {
		firing = append(firing, e.Shooter.SteamID64)
	})
	// 	fmt.Printf("Round over %s \n", e.Message)

	// })

	// p.RegisterEventHandler(func(e events.BombPlanted) {
	// 	fmt.Printf("%s Planted the bomb \n", e.Player)
	// })

	// p.RegisterEventHandler(func(e events.Kill) {
	// 	var hs string
	// 	if e.IsHeadshot {
	// 		hs = " (HS)"
	// 	}
	// 	var wallBang string
	// 	if e.PenetratedObjects > 0 {
	// 		wallBang = " (WB)"
	// 	}
	// 	fmt.Printf("%s <%v%s%s> %s\n", e.Killer, e.Weapon, hs, wallBang, e.Victim)
	// })
	// Parse to end
	err = p.ParseToEnd()

	var frameRate int = int(p.Header().PlaybackFrames / int(p.Header().PlaybackTime.Seconds()))

	var game = Game{
		Players:        players,
		Map:            p.Header().MapName,
		Frames:         frames,
		GameStartFrame: gameStartFrame,
		FrameRate:      frameRate,
	}
	var gameModel = model.Game{
		Map:       &game.Map,
		FrameRate: &game.FrameRate,
		Frames:    game.Frames,
		Players:   game.Players,
	}
	// jsonObj, err := json.Marshal(game)

	// err = os.WriteFile("./output.json", jsonObj, 0644)
	// // fmt.Println(string(b))
	if err != nil {
		log.Panic("failed to parse demo: ", err)
	}
	return gameModel
}
