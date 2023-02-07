package main

import (
	"encoding/json"
	"log"
	"os"

	"github.com/golang/geo/r3"
	dem "github.com/markus-wa/demoinfocs-golang/v3/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v3/pkg/demoinfocs/events"
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
}

type FrameState struct {
	Frame        int           `json:"frame"`
	PlayerStates []PlayerState `json:"playerState"`
}

func main() {
	f, err := os.Open("./test.dem")
	if err != nil {
		log.Panic("failed to open demo file: ", err)
	}
	defer f.Close()

	p := dem.NewParser(f)
	defer p.Close()
	var frames []FrameState

	p.RegisterEventHandler(func(e events.FrameDone) {

		// TODO get state for all players at any frame
		var participants = p.GameState().Participants().Playing()
		var playerStates []PlayerState
		for _, element := range participants {
			playerStates = append(playerStates,
				PlayerState{
					Name:        element.Name,
					Kills:       element.Kills(),
					Deaths:      element.Deaths(),
					Assists:     element.Assists(),
					Mvps:        element.MVPs(),
					SteamId:     element.SteamID64,
					Position:    element.Position(),
					EyePosition: element.PositionEyes(),
				})
		}

		frames = append(frames, FrameState{Frame: p.CurrentFrame(), PlayerStates: playerStates})

	})

	// p.RegisterEventHandler(func(e events.MatchStart) {
	// 	fmt.Println("Game started --------------------------------------------------")
	// })

	// p.RegisterEventHandler(func(e events.RoundStart) {
	// 	fmt.Println("New round ------------------------------------------------------ ")
	// })
	// p.RegisterEventHandler(func(e events.RoundEnd) {

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
	b, err := json.Marshal(frames)
	err = os.WriteFile("./output.json", b, 0644)
	// fmt.Println(string(b))
	// fmt.Println(frames[10000])
	if err != nil {
		log.Panic("failed to parse demo: ", err)
	}
}
