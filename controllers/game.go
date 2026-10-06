package controllers

import (
	"fmt"
	"io"
	"log"
	"math"
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
	Round   *int          `json:"round"`
	Frames  []FrameState  `json:"frames"`
	Winner  string        `json:"winner"`
	Kills   []KillEvent   `json:"kills"`
	Economy *RoundEconomy `json:"economy,omitempty"` // captured at freeze-time end
}

// PlayerEconomy is one player's money situation for a round, captured at
// freeze-time end when the buy phase is over.
type PlayerEconomy struct {
	Name       string `json:"name"`
	SteamId    string `json:"steamId"`
	StartMoney int    `json:"startMoney"`       // bank at round start, before buys
	Spent      int    `json:"spent"`            // money spent during the buy phase
	EquipValue int    `json:"equipValue"`       // equipment value at freeze-time end
	Bank       int    `json:"bank"`             // money remaining after the buy
	Weapon     string `json:"weapon,omitempty"` // primary weapon held ("" = default pistol only)
	Armor      int    `json:"armor"`            // 0-100
	Helmet     bool   `json:"helmet"`
	DefuseKit  bool   `json:"defuseKit"`
}

// TeamEconomy aggregates a team's buy for one round.
type TeamEconomy struct {
	Players    []PlayerEconomy `json:"players"`
	AvgEquip   int             `json:"avgEquip"`            // average equipment value after the buy
	AvgSpent   int             `json:"avgSpent"`            // average money spent in the buy phase
	TotalSpent int             `json:"totalSpent"`          // total money spent in the buy phase
	Type       string          `json:"type"`                // heuristic buy classification
	Rifles     int             `json:"rifles,omitempty"`    // players holding a rifle-class weapon
	Survivors  int             `json:"survivors,omitempty"` // players who kept weapons from the previous round
}

// RoundEconomy holds both teams' buy for one round. It is the input for the
// JEV round analysis (controllers/jev.go).
type RoundEconomy struct {
	CT *TeamEconomy `json:"ct,omitempty"`
	T  *TeamEconomy `json:"t,omitempty"`
}

// buyType classifies a team's round from held rifles (strength), money spent
// (intent) and equipment value. It serves as the deterministic baseline that
// the JEV analysis compares its semantic judgment against (see controllers/jev.go).
//   - full:  4+/5 players hold rifle-class weapons (bought or kept) — the
//     strength matters, not whether the guns were bought this round
//   - kept:  full rifle strength without a meaningful buy (< $1500 per player:
//     armor and utility top-ups don't turn a kept round into a full one)
//   - hero:  exactly one player bought a real weapon (>= $1500: rifle, scout,
//     or Deagle+armor territory) while the rest saved
//   - eco:   bought nothing meaningful (< $2000 equip AND < $1000 spent)
//   - force: cheap weapons (SMGs, pistols with armor) — under $3500 equip
//   - half:  only 2-3 players hold rifles — the genuine mixed buy
func buyType(pistol, hero bool, rifles, teamSize, avgEquip, avgSpent int) string {
	switch {
	case pistol:
		return "pistol"
	case hero:
		return "hero"
	case rifles >= teamSize-1:
		// (near-)full rifle strength; money flow tells kept from full
		if avgSpent < 1500 {
			return "kept"
		}
		return "full"
	case avgEquip < 2000 && avgSpent < 1000:
		return "eco"
	case avgEquip < 3500:
		return "force"
	default:
		return "half"
	}
}

// pistolRound reports whether every player present at round start began the
// round with the standard $800 pistol-round money.
func pistolRound(roundStartMoney map[uint64]int) bool {
	if len(roundStartMoney) == 0 {
		return false
	}
	for _, m := range roundStartMoney {
		if m != 800 {
			return false
		}
	}
	return true
}

// defaultPistols are the free pistols everyone spawns with — holding one
// says nothing about the buy.
var defaultPistols = map[common.EquipmentType]bool{
	common.EqGlock: true,
	common.EqUSP:   true,
	common.EqP2000: true,
}

// primaryWeapon returns the most meaningful weapon in the player's inventory
// — the primary weapon (rifle / SMG / heavy) if present, otherwise a notable
// (bought) pistol such as a Deagle or P250, or "" when they hold only a
// default pistol or nothing. The bool reports whether they hold a
// rifle-class weapon (EqClassRifle includes the AWP and SSG 08).
func primaryWeapon(m *common.Player) (string, bool) {
	var pistol *common.Equipment
	for _, eq := range m.Inventory {
		if eq == nil {
			continue
		}
		switch eq.Class() {
		case common.EqClassRifle:
			return eq.String(), true
		case common.EqClassSMG, common.EqClassHeavy:
			return eq.String(), false
		case common.EqClassPistols:
			pistol = eq
		}
	}
	if pistol != nil && !defaultPistols[pistol.Type] {
		return pistol.String(), false
	}
	return "", false
}

// snapshotTeamEconomy captures a team's buy one frame after freeze time
// ended, when all freeze-time-end property updates are flushed.
//
// CS2 gotchas this works around (verified against a GOTV demo):
//   - Player.MoneySpentThisRound() does not reset between rounds
//   - Player.EquipmentValueFreezeTimeEnd() is one round stale if read inside
//     the RoundFreezetimeEnd handler itself
//
// Money at round start is captured separately on RoundStart (money lives on
// the controller entity, so pawn recreation at round start doesn't affect
// it); spent is then start money minus bank.
func snapshotTeamEconomy(team *common.TeamState, pistol bool, roundStartMoney map[uint64]int, prevSurvivors map[uint64]bool) *TeamEconomy {
	if team == nil {
		return nil
	}
	var players []PlayerEconomy
	totalEquip, totalSpent, survivors, rifles := 0, 0, 0, 0
	for _, m := range team.Members() {
		if m.SteamID64 == 0 && m.Name == "" {
			continue // skip empty slots
		}
		if prevSurvivors[m.SteamID64] {
			survivors++
		}
		start, ok := roundStartMoney[m.SteamID64]
		if !ok {
			// joined after round start — no start snapshot, best effort
			start = m.Money()
		}
		bank := m.Money()
		spent := start - bank
		if spent < 0 {
			spent = 0
		}
		weapon, hasRifle := primaryWeapon(m)
		if hasRifle {
			rifles++
		}
		pe := PlayerEconomy{
			Name:       m.Name,
			SteamId:    strconv.FormatUint(m.SteamID64, 10),
			StartMoney: start,
			Spent:      spent,
			EquipValue: m.EquipmentValueCurrent(),
			Bank:       bank,
			Weapon:     weapon,
			Armor:      m.Armor(),
			Helmet:     m.HasHelmet(),
			DefuseKit:  m.HasDefuseKit(),
		}
		players = append(players, pe)
		totalEquip += pe.EquipValue
		totalSpent += pe.Spent
	}
	if len(players) == 0 {
		return nil
	}
	avg := totalEquip / len(players)
	avgSpent := totalSpent / len(players)
	// hero detection: exactly one player bought a real weapon (>= $1500 —
	// rifle, scout or Deagle+armor territory) while the rest saved (< $1000),
	// and the team isn't holding kept rifles
	buyers, savers := 0, 0
	for _, pe := range players {
		if pe.Spent >= 1500 {
			buyers++
		} else if pe.Spent < 1000 {
			savers++
		}
	}
	hero := buyers == 1 && savers == len(players)-1 && avg < 3500
	return &TeamEconomy{
		Players:    players,
		AvgEquip:   avg,
		AvgSpent:   avgSpent,
		TotalSpent: totalSpent,
		Type:       buyType(pistol, hero, rifles, len(players), avg, avgSpent),
		Rifles:     rifles,
		Survivors:  survivors,
	}
}

// captureRoundEconomy builds the economy snapshot for the current round from
// the live game state.
func captureRoundEconomy(gs dem.GameState, roundStartMoney map[uint64]int, prevSurvivors map[uint64]bool) *RoundEconomy {
	pistol := pistolRound(roundStartMoney)
	return &RoundEconomy{
		CT: snapshotTeamEconomy(gs.TeamCounterTerrorists(), pistol, roundStartMoney, prevSurvivors),
		T:  snapshotTeamEconomy(gs.TeamTerrorists(), pistol, roundStartMoney, prevSurvivors),
	}
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
	// Storage bookkeeping first: the upload flow already created the row;
	// the -parse/-reparse CLI paths create it here (name = id, like the old
	// meta-less "local" demo). Clearing rounds + caches up front means a
	// fresh parse never inherits stale derived data — same contract the
	// analysis.json/postplant.json removals below used to carry.
	if exists, err := db.DemoExists(demoId); err != nil {
		log.Panic("failed to look up demo row: ", err)
	} else if !exists {
		if err := db.CreateDemo(demoId, demoId); err != nil {
			log.Panic("failed to create demo row: ", err)
		}
	}
	if err := db.ClearDemoRounds(demoId); err != nil {
		log.Panic("failed to clear old rounds: ", err)
	}

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
	pendingEconomy := map[int]*RoundEconomy{} // economy waiting for its round struct to exist
	roundStartMoney := map[uint64]int{}       // bank at round start, before buys
	prevSurvivors := map[uint64]bool{}        // players alive at the previous RoundEnd (they keep their weapons)
	economyPending := false                   // freeze ended; capture economy on the next frame
	// Live play is tracked by round lifecycle events, NOT GamePhase():
	// GamePhase==2 (StartGame) is only set once per half in CS2, so the old
	// phase heuristic never fired in the second half.
	livePhase := false
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
		if livePhase {
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

		// The buy phase just ended (see the RoundFreezetimeEnd handler): capture
		// the economy now, one frame later, when all property updates are
		// flushed.
		if economyPending {
			economyPending = false
			econ := captureRoundEconomy(p.GameState(), roundStartMoney, prevSurvivors)
			if round < len(rounds) {
				rounds[round].Economy = econ
			} else {
				pendingEconomy[round] = econ
			}
		}

		if len(rounds) <= round {
			econ := pendingEconomy[round]
			delete(pendingEconomy, round)
			rounds = append(rounds, Round{
				Round:   &round,
				Kills:   roundKills[round],
				Economy: econ,
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
		pendingEconomy = map[int]*RoundEconomy{}
		roundStartMoney = map[uint64]int{}
		prevSurvivors = map[uint64]bool{}
		economyPending = false
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
			// Money lives on the controller entity and is already updated with
			// round rewards by now: snapshot the bank before any buys.
			roundStartMoney = map[uint64]int{}
			for _, m := range p.GameState().Participants().Playing() {
				roundStartMoney[m.SteamID64] = m.Money()
			}
		}
		livePhase = false // freeze time
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
		livePhase = false // round-over period

		// Players alive right now keep their weapons into the next round.
		// (Deaths during the round-over period, after RoundEnd, are rare and
		// would incorrectly count as survivors — accepted inaccuracy.)
		if matchStarted {
			prevSurvivors = map[uint64]bool{}
			for _, m := range p.GameState().Participants().Playing() {
				if m.SteamID64 != 0 && m.IsAlive() {
					prevSurvivors[m.SteamID64] = true
				}
			}
		}

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

	p.RegisterEventHandler(func(e events.RoundFreezetimeEnd) {
		// Freeze time is over and the buy phase is done. The economy is captured
		// one frame later (see the FrameDone handler) because reading
		// equipment properties inside this handler returns stale values in
		// CS2 demos.
		if matchStarted && currentRound >= 0 {
			economyPending = true
			livePhase = true // live play until RoundEnd
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
	// Measure the real frame rate from the recorded frames — demoinfocs'
	// header values are unavailable mid-parse for CS2. Use LIVE-phase frames
	// of the round with the most live play: the demo tick rate is exact there
	// (~67/s for CS2), while round-over periods record sparser and would skew
	// the measurement down.
	frameRate := 60
	bestLive, bestLiveIdx := -1, -1
	for i, r := range rounds {
		n := 0
		for _, f := range r.Frames {
			if f.Phase == "LIVE" {
				n++
			}
		}
		if n > bestLive {
			bestLive, bestLiveIdx = n, i
		}
	}
	if bestLiveIdx >= 0 {
		var live []FrameState
		for _, f := range rounds[bestLiveIdx].Frames {
			if f.Phase == "LIVE" {
				live = append(live, f)
			}
		}
		if len(live) > 1 {
			if dt := live[len(live)-1].Time - live[0].Time; dt > 0 {
				frameRate = int(math.Round(float64(len(live)-1) / dt))
			}
		}
	}
	game := Game{
		Players:        players,
		Map:            mapName,
		Rounds:         rounds,
		GameStartFrame: gameStartFrame,
		GameEndFrame:   gameEndFrame,
		FrameRate:      frameRate,
		RoundCount:     len(rounds),
	}
	fmt.Println("Parsed game, frame rate:", frameRate)
	// Any legacy JSON tree for this demo is now stale (the store is the
	// only read path) — drop it so a deleted-database recovery can never
	// resurrect old output. Non-legacy demos simply have no directory.
	_ = os.RemoveAll(demoDir(demoId))
	for i := range game.Rounds {
		fmt.Println("round", i+1, "/", len(game.Rounds))
	}

	// The parse result: rounds (frame blobs + derived columns + kills) and
	// the demo metadata row, all in one go. Derived analyses (JEV,
	// post-plant) were cleared with the old rounds above.
	if err := db.InsertRounds(demoId, game.Rounds); err != nil {
		log.Panic("failed to store rounds: ", err)
	}
	if err := db.FinishDemo(demoId, game); err != nil {
		log.Panic("failed to finalize demo row: ", err)
	}
	return game
}
