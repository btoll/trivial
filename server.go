package trivial

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"text/template"

	"github.com/btoll/trivial/middleware"
	"golang.org/x/net/websocket"
)

//go:embed templates/*.gohtml
var templateFiles embed.FS

// A socket server instance is set up to handle
// multiple (concurrent) games.
type SocketServer struct {
	Location URL
	Games    map[string]*Game
	Tpl      *template.Template
	Mux      *http.ServeMux
}

func NewSocketServer(url URL) *SocketServer {
	return &SocketServer{
		Location: url,
		Games:    make(map[string]*Game),
		// In templates/, the `_base.html` file **must** be the first file!!
		// The underscore (_) is lexically before any lowercase alpha character,
		// **do not** remove it!!!  Everything will break!!!
		Tpl: template.Must(template.ParseFS(templateFiles, "templates/*.gohtml")),
		Mux: http.NewServeMux(),
	}
}

type Socket struct {
	Protocol string
	Domain   string
	Port     int
}

func (s Socket) String() string {
	return fmt.Sprintf("%s://%s:%d",
		s.Protocol,
		s.Domain,
		s.Port,
	)
}

type URL struct {
	Sock Socket
	Path string
}

func (u URL) String() string {
	return fmt.Sprintf("%s://%s:%d/%s",
		u.Sock.Protocol,
		u.Sock.Domain,
		u.Sock.Port,
		u.Path,
	)
}

// This is marshaled to the browser client.
// See [SocketServer.Publish].
type ServerMessage struct {
	Type string `json:"type,omitempty"`
	Data any    `json:"data,omitempty"`
}

// The socket server unmarshals the response from the
// browser client into this type.
type ClientMessage struct {
	Type     string `json:"type,omitempty"`
	Username string `json:"username,omitempty"`
	Token    string `json:"token,omitempty"`
	Data     any    `json:"data,omitempty"`
}

// A socket server instance can potentially have multiple games.
// Note this only checks for token equality **not** expiration.
func (s *SocketServer) GetGame(key string) (*Game, error) {
	if key == "" {
		return nil, errors.New("API key is an empty string")
	}
	if game, ok := s.Games[key]; ok {
		return game, nil
	}
	return nil, fmt.Errorf("game `%s` not found", key)
}

// When a connection is suddenly disconnected, for instance
// when the browser crashes, we don't have any information
// about the player that closed the session other than the
// socket. This means that we need to range over all of the
// games and the players within each game until we find the
// matching player.
func (s *SocketServer) GetPlayerBySocket(socket *websocket.Conn) (*Player, *Game, error) {
	for _, game := range s.Games {
		player, _ := game.GetPlayer(socket)
		if player != nil {
			return player, game, nil
		}
	}
	return nil, nil, errors.New("cannot get player from socket")
}

// Notify a single player of an event.
func (s *SocketServer) Message(socket *websocket.Conn, msg ServerMessage) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = socket.Write(b)
	if err != nil {
		return fmt.Errorf("websocket write error: %v", err)
	}
	return nil
}

// Notifies every player of an event.
func (s *SocketServer) Publish(game *Game, msg ServerMessage) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	for _, player := range game.Players {
		go func(player Player) {
			if _, err := player.Socket.Write(b); err != nil {
				fmt.Println("websocket write error:", err)
			}
		}(*player)
	}
	return nil
}

func (s *SocketServer) RegisterAndStartGame(game *Game) {
	s.RegisterGame(game)
	s.StartGame(game)
}

// Registers a new game. A socket server can host multiple games.
func (s *SocketServer) RegisterGame(game *Game) {
	s.Games[game.Key.Key] = game
}

// Registers all the handlers with the new mux, adds the middleware
// and starts starts the game server.
func (s *SocketServer) StartGame(game *Game) {
	s.Mux.Handle("/ws", websocket.Handler(s.DefaultHandler))
	s.Mux.HandleFunc("/", s.BaseHandler)
	s.Mux.HandleFunc("/health", s.HealthHandler)
	s.Mux.HandleFunc("/kill", s.KillHandler)
	s.Mux.HandleFunc("/message", s.MessageHandler)
	s.Mux.HandleFunc("/notify", s.NotifyHandler)
	s.Mux.HandleFunc("/query", s.QueryHandler)
	s.Mux.HandleFunc("/reset", s.ResetHandler)
	s.Mux.HandleFunc("/scoreboard", s.ScoreboardHandler)
	s.Mux.HandleFunc("/update_score", s.UpdateScoreHandler)
	//	log.Fatal(http.ListenAndServe(":3000", middleware.NewLogger(NewAuthenticator(&game.Key, s.Mux))))
	log.Fatal(http.ListenAndServeTLS(":3000", "cert.pem", "key.pem", middleware.NewLogger(middleware.NewAuthenticator(&game.Key, s.Mux))))
}
