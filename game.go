package trivial

import (
	"errors"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/btoll/trivial/middleware"
	"golang.org/x/net/websocket"
)

type GamePlayers []*Player

// Players can have a zero `Score`, so don't add the tag
// `omitempty` when marshaling to the browser.
//
// The `Socket` is the only reliable way to lookup a particular
// player, and the functions in the package operate on it as
// often as it can.
// For example, the `player.Name` could be fiddled with in the
// browser before sending a request so it could be unreliable.
//
// The `UUID` is set by the client (browser) and sent
// as part of the websocket URL. It's not currently used.
//
//	const socketURL = `{{ . }}?uuid=${getUUID()}`;
//	socket = new WebSocket(socketURL);
type Player struct {
	Location string          `json:"location,omitempty"`
	Name     string          `json:"name,omitempty"`
	UUID     string          `json:"uuid,omitempty"`
	Score    int             `json:"score"`
	Socket   *websocket.Conn `json:"conn,omitempty"`
}

type Scoreboard []*PlayerScore

func (s Scoreboard) Len() int {
	return len(s)
}

func (s Scoreboard) Less(i, j int) bool {
	return s[i].Score > s[j].Score
}

func (s Scoreboard) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// This is currently for an admin to get a quick view
// of the game state.
type PlayerScore struct {
	Name  string
	Score int
}

// `Weight` is the amount of points awarded for a
// correct answer.
type CurrentQuestion struct {
	Question  string   `json:"question,omitempty"`
	Answer    any      `json:"answer,omitempty"`
	Choices   []string `json:"choices,omitempty"`
	Weight    int      `json:"weight,omitempty"`
	Responses int      `json:"responses,omitempty"`
}

type Game struct {
	Name    string
	Players GamePlayers
	Benched GamePlayers
	Key     middleware.APIKey
	CurrentQuestion
}

func has(pool GamePlayers, v any) (int, *Player) {
	switch vv := v.(type) {
	case string:
		for i, player := range pool {
			if vv == player.Name {
				return i, player
			}
		}
	case *Player:
		for i, player := range pool {
			if vv == player {
				return i, player
			}
		}
	}
	return -1, nil
}

func remove(pool GamePlayers, index int) GamePlayers {
	return append(pool[:index], pool[index+1:]...)
}

// https://play.golang.com/p/yG1ouhTriOP
func toInt(b []byte) (int, error) {
	//	result := 0
	//	for i := 0; i < len(bytes); i++ {
	//		result = result << 8
	//		result += int(bytes[i])
	//
	//	}
	//	return result
	s := string(b)
	i, err := strconv.Atoi(s)
	if err != nil {
		return -1, err
	}
	return i, nil
}

// Constructor.
func NewGame(name string, tokenExpiration float64) *Game {
	return &Game{
		Name:    name,
		Players: make(GamePlayers, 0),
		Key:     middleware.GenerateKey(name, tokenExpiration),
	}
}

// This function will move a `Player` out of the game's `Players`
// pool and into the Benched pool of the [Game] type. This will
// occur when a connection is lost due to the browser tab being closed.
// Since this could have occurred by accident (laptop shuts down, etc),
// the player should be able to log back into the game and resume
// where they left off, that is reclaim the points they had when the
// disconnect occurred.
// Note that this is different from a player choosing to exit the game
// by clicking exit or close (TODO).
func (g *Game) Bench(p *Player) error {
	n, player := has(g.Players, p)
	if n == -1 {
		return errors.New("Player not found.")
	}
	g.Players = remove(g.Players, n)
	g.Benched = append(g.Benched, player)
	return nil
}

// Called only when a new player logs in. It is legal for
// a logged in player to continue making requests after
// the game has expired, but not if they have not previously
// logged in.
// See [Game.CheckTokenEquality] for more information.
func (g *Game) CheckTokenExpiration() error {
	if g.Key.Expired {
		return errors.New("API key has already expired")
	}
	since := g.Key.TimeCreated.Sub(time.Now().UTC())
	if math.Abs(since.Seconds()) > g.Key.Expiration {
		g.Key.Expired = true
		return errors.New("API key has expired")
	}
	return nil
}

// This function expects either a player name (string) or
// a player socket (*websocket.Conn).
// The most reliable way to lookup a player is by their
// socket, since this cannot be modified by the user. However,
// when calling an endpoint such as [SocketServer.KillHandler],
// all we have is the player name.
func (g *Game) GetPlayer(v any) (*Player, error) {
	switch vv := v.(type) {
	case string:
		for _, player := range g.Players {
			if vv == player.Name {
				return player, nil
			}
		}
	case *websocket.Conn:
		for _, player := range g.Players {
			if vv == player.Socket {
				return player, nil
			}
		}
	}
	return nil, errors.New("Player not found.")
}

func (g *Game) GetScoreboard() Scoreboard {
	scoreboard := make(Scoreboard, len(g.Players))
	for i, player := range g.Players {
		scoreboard[i] = &PlayerScore{
			Name:  player.Name,
			Score: player.Score,
		}
	}
	sort.Sort(scoreboard)
	return scoreboard
}

func (g *Game) HasPlayer(name string) (*Player, bool) {
	if b, player := has(g.Benched, name); b > -1 {
		return player, true
	}
	if b, player := has(g.Players, name); b > -1 {
		return player, false
	}
	return nil, false
}

// If a player logs back in after accidentally killing
// their browser session (at which point they are "benched"),
// move their player state from the .Benched pool to
// the .Players pool in the [Game] type.
// This has the effect of allowing them to resume where they
// left off and regain their points.
func (g *Game) Unbench(p *Player) error {
	n, player := has(g.Benched, p)
	if n == -1 {
		return errors.New("Player not found.")
	}
	g.Benched = remove(g.Benched, n)
	g.Players = append(g.Players, player)
	return nil
}

// Called every time a player guesses correctly. Currently,
// this happens immediately after a correct guess and so
// every player will see the updated score.
// This isn't optimal and should be changed to only update
// after everyone has guessed (TODO).
func (g *Game) UpdatePlayerScore(socket *websocket.Conn, points int) (int, error) {
	for i, player := range g.Players {
		if socket == player.Socket {
			total := points + player.Score
			g.Players[i].Score = total
			return total, nil
		}
	}
	return 0, errors.New("Player not found.")
}
