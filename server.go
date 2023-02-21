package trivial

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"golang.org/x/net/websocket"
)

//go:embed templates/*.gohtml
var templateFiles embed.FS

// A socket server instance is set up to handle
// multiple (concurrent) games.
type SocketServer struct {
	Location URI
	Games    map[string]*Game
	Tpl      *template.Template
}

func NewSocketServer(uri URI) *SocketServer {
	fmt.Printf("created new websocket server `%s`\n", uri)
	return &SocketServer{
		Location: uri,
		Games:    make(map[string]*Game),
		// `_base.html` file **must** be the first file!!
		// The underscore (_) is lexically before any lowercase alpha character,
		// **do not** remove it!
		Tpl: template.Must(template.ParseFS(templateFiles, "templates/*.gohtml")),
	}
}

type Socket struct {
	Protocol string
	Domain   string
	Port     int
}

type URI struct {
	Sock Socket
	Path string
}

func (u URI) String() string {
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
	Type string      `json:"type,omitempty"`
	Data interface{} `json:"data,omitempty"`
}

// The socket server unmarshals the response from the
// browser client into this type.
type ClientMessage struct {
	Type     string      `json:"type,omitempty"`
	Username string      `json:"username,omitempty"`
	Token    string      `json:"token,omitempty"`
	Data     interface{} `json:"data,omitempty"`
}

func (s *SocketServer) BaseHandler(w http.ResponseWriter, r *http.Request) {
	r.Header = http.Header{
		"Content-Type": {"text/html; charset=utf-8"},
	}

	if err := s.Tpl.Execute(w, s.Location); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *SocketServer) DefaultHandler(socket *websocket.Conn) {
	buf := make([]byte, 1024)
	origin := socket.Config().Origin
	location := socket.Config().Location

	fmt.Println("incoming connection from client", location)

	for {
		n, err := socket.Read(buf)
		if err != nil {
			if err == io.EOF {
				fmt.Println(origin)
				// This means the client connection has closed.
				// Unfortunately, we don't have any information about
				// who closed the session other than the socket.
				// This means that we need to range over all of the games
				// and the players within each game until we find the
				// matching player.
				player, game, err := s.GetPlayerBySocket(socket)
				if err != nil {
					fmt.Println("read error:", err)
				} else {
					fmt.Printf("%s just left the building\n", player.Name)
					game.Bench(player)
					err = s.Publish(game, ServerMessage{
						Type: "player_delete",
						Data: game.Players,
					})
					if err != nil {
						log.Fatalln(err)
					}
				}
				break
			}
			fmt.Println("read error:", err)
			// Don't return here, b/c it will break the connection.
			continue
		}

		data := buf[:n]

		var msg ClientMessage
		err = json.Unmarshal(data, &msg)
		if err != nil {
			log.Fatalln(err)
		}

		// `getGame` will verify the **equality** of the token
		// **not** if it has expired.
		// Not checking for expiration here allows those players
		// who've already logged in to continue, but will disallow
		// new players from joining (see the "login" case below).
		game, err := s.GetGame(msg.Token)
		if err != nil {
			b, err := json.Marshal(ServerMessage{
				Type: "error",
				Data: fmt.Sprintf("There has been a problem accessing game `%s`", msg.Token),
			})
			if err != nil {
				fmt.Println("marshall error:", err)
			} else {
				socket.Write(b)
			}
		} else {
			switch msg.Type {
			case "login":
				username := strings.TrimSpace(msg.Username)
				player, benched := game.HasPlayer(msg.Username)
				if player != nil {
					if benched {
						game.Unbench(player)
						player.Socket = socket
						err = s.Publish(game, ServerMessage{
							Type: "player_add",
							Data: game.Players,
						})
						if err != nil {
							log.Fatalln(err)
						}
					} else {
						b, err := json.Marshal(ServerMessage{
							Type: "error",
							Data: fmt.Sprintf("Username `%s` exists, choose another", username),
						})
						if err != nil {
							fmt.Println("marshall error:", err)
						} else {
							// TODO: check if this actually sent?
							socket.Write(b)
						}
					}
				} else {
					fmt.Println("received data from client", string(data))
					err = game.CheckTokenExpiration()
					if err != nil {
						b, err := json.Marshal(ServerMessage{
							Type: "error",
							Data: fmt.Sprintf("%s", "Game has expired"),
						})
						if err != nil {
							log.Fatalln(err)
						}
						socket.Write(b)
					} else {
						parsedUrl, err := url.Parse(fmt.Sprintf("%s", location))
						if err != nil {
							fmt.Println("url.Parse error:", err)
						}
						uuid := strings.Split(parsedUrl.RawQuery, "=")
						newPlayer := &Player{
							Location: fmt.Sprintf("%s", origin),
							Name:     username,
							UUID:     uuid[1],
							Score:    0,
							Socket:   socket,
						}
						game.Players = append(game.Players, newPlayer)
						err = s.Publish(game, ServerMessage{
							Type: "player_add",
							Data: game.Players,
						})
						if err != nil {
							log.Fatalln(err)
						}
					}
				}
			case "guess":
				// Note that we're also doing this above.
				// Should this be done for every received message?
				player, err := game.GetPlayer(socket)
				if err != nil {
					fmt.Println("read error:", err)
				}

				// First, message the player individually if the answer was correct (or not).
				// TODO: This probably needs revisited...
				res := true
				switch vv := msg.Data.(type) {
				case string:
					res = game.CurrentQuestion.Answer == vv
				case []interface{}:
					correctAnswers := strings.Split(game.CurrentQuestion.Answer, ",")
					sort.Strings(correctAnswers)
					if len(correctAnswers) != len(vv) {
						res = false
						break
					}
					for i, char := range vv {
						if char != correctAnswers[i] {
							res = false
							break
						}
					}
				}
				// Second, if the answer is correct, update everyone.
				b, err := json.Marshal(ServerMessage{
					Type: "player_message",
					Data: res,
				})
				if err != nil {
					fmt.Println("marshall error:", err)
				} else {
					// TODO: check if this actually sent?
					socket.Write(b)
				}
				if res {
					_, err := game.UpdatePlayerScore(socket, game.CurrentQuestion.Weight)
					if err != nil {
						log.Fatalln(err)
					}
					fmt.Printf("%s correctly guessed %s, %d current points\n",
						player.Name,
						msg.Data,
						player.Score)
					err = s.Publish(game, ServerMessage{
						Type: "update_scoreboard",
						Data: game.Players,
					})
					if err != nil {
						log.Fatalln(err)
					}
				} else {
					fmt.Printf("%s incorrectly guessed %s, %d current points\n",
						player.Name,
						msg.Data,
						player.Score)
				}
			case "question":
				//todo
			}
		}
	}
}

// A socket server instance can potentially have multiple games.
// Note this only checks for token equality **not** expiration.
func (s *SocketServer) GetGame(key string) (*Game, error) {
	if key == "" {
		return nil, fmt.Errorf("API key is an empty string")
	}
	if game, ok := s.Games[key]; ok {
		err := game.CheckTokenEquality(key)
		if err != nil {
			return game, err
		}
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
	return nil, nil, fmt.Errorf("cannot get player from socket")
}

func (s *SocketServer) KillHandler(w http.ResponseWriter, r *http.Request) {
	game, err := s.GetGame(r.Header.Get("X-TRIVIA-APIKEY"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	parsedUrl, err := url.Parse(fmt.Sprintf("%s", r.URL))
	if err != nil {
		fmt.Println("url.Parse error:", err)
	}
	p := strings.Split(parsedUrl.RawQuery, "=")
	player, err := game.GetPlayer(p[1])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	m, err := json.Marshal(ServerMessage{
		Type: "logout",
		Data: "",
	})
	if _, err := player.Socket.Write(m); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	game.Bench(player)
	fmt.Println("killing player", player.Name)
}

func (s *SocketServer) MessageHandler(w http.ResponseWriter, r *http.Request) {
	game, err := s.GetGame(r.Header.Get("X-TRIVIA-APIKEY"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	parsedUrl, err := url.Parse(fmt.Sprintf("%s", r.URL))
	if err != nil {
		fmt.Println("url.Parse error:", err)
	}
	p := strings.Split(parsedUrl.RawQuery, "=")
	player, err := game.GetPlayer(p[1])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	b, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	m, err := json.Marshal(ServerMessage{
		Type: "notify_player",
		Data: string(b),
	})
	if _, err := player.Socket.Write(m); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *SocketServer) NotifyHandler(w http.ResponseWriter, r *http.Request) {
	game, err := s.GetGame(r.Header.Get("X-TRIVIA-APIKEY"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	b, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	err = s.Publish(game, ServerMessage{
		Type: "notify_all",
		Data: fmt.Sprintf("%s", b),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
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

func (s *SocketServer) QueryHandler(w http.ResponseWriter, r *http.Request) {
	game, err := s.GetGame(r.Header.Get("X-TRIVIA-APIKEY"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	b, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	l := strings.Split(fmt.Sprintf("%s", b), "|")
	weight, err := strconv.Atoi(l[2])
	if err != nil {
		http.Error(w, fmt.Sprintf("%s", err), http.StatusInternalServerError)
		return
	}
	var choices []string
	if len(l[0]) > 3 {
		choices = l[3:]
	}
	game.CurrentQuestion = CurrentQuestion{
		Question: l[0],
		// TODO: probably don't want to send the answer to the client.
		Answer:  l[1],
		Choices: choices,
		Weight:  weight,
	}
	b, err = json.Marshal(game.CurrentQuestion)
	if err != nil {
		fmt.Println(err)
	}
	err = s.Publish(game, ServerMessage{
		Type: "question",
		Data: string(b),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Dump the current question to `stdout` (and later to a log).
	b, err = json.MarshalIndent(game.CurrentQuestion, "", "    ")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(b))
}

// Registers a new game. A socket server can host multiple games.
func (s *SocketServer) RegisterGame(game *Game) {
	s.Games[game.Key.Key] = game
	fmt.Printf("registered game `%s` with key `%s`\n", game.Name, game.Key.Key)
}

func (s *SocketServer) ResetHandler(w http.ResponseWriter, r *http.Request) {
	game, err := s.GetGame(r.Header.Get("X-TRIVIA-APIKEY"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for i := range game.Players {
		game.Players[i].Score = 0
	}
	err = s.Publish(game, ServerMessage{
		Type: "update_scoreboard",
		Data: game.Players,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *SocketServer) ScoreboardHandler(w http.ResponseWriter, r *http.Request) {
	game, err := s.GetGame(r.Header.Get("X-TRIVIA-APIKEY"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Dump the current question to `stdout` (and later to a log).
	//	b, err := json.MarshalIndent(game.getScoreboard(), "", "    ")
	b, err := json.Marshal(game.GetScoreboard())
	if err != nil {
		fmt.Println(err)
	}
	fmt.Fprintln(w, string(b))
}
