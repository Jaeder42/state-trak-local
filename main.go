package main

import (
	"fmt"
	"log"
	"os"

	dem "github.com/markus-wa/demoinfocs-golang/v3/pkg/demoinfocs"
	events "github.com/markus-wa/demoinfocs-golang/v3/pkg/demoinfocs/events"
)

func main() {
	f, err := os.Open("./test.dem")
	if err != nil {
		log.Panic("failed to open demo file: ", err)
	}
	defer f.Close()

	p := dem.NewParser(f)
	defer p.Close()

	// Register handler on kill events

	p.RegisterEventHandler(func(e events.MatchStart) {
		fmt.Println("Game started --------------------------------------------------")
	})

	p.RegisterEventHandler(func(e events.RoundStart) {
		fmt.Println("New round ------------------------------------------------------ ")
	})
	p.RegisterEventHandler(func(e events.RoundEnd) {

		fmt.Printf("Round over %s \n", e.Message)

	})

	p.RegisterEventHandler(func(e events.BombPlanted) {
		fmt.Printf("%s Planted the bomb \n", e.Player)
	})

	p.RegisterEventHandler(func(e events.Kill) {
		var hs string
		if e.IsHeadshot {
			hs = " (HS)"
		}
		var wallBang string
		if e.PenetratedObjects > 0 {
			wallBang = " (WB)"
		}
		fmt.Printf("%s <%v%s%s> %s\n", e.Killer, e.Weapon, hs, wallBang, e.Victim)
	})

	// Parse to end
	err = p.ParseToEnd()
	if err != nil {
		log.Panic("failed to parse demo: ", err)
	}
}
