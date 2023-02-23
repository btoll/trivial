package trivial

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/websocket"
)

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
					fmt.Println(err)
				} else {
					// TODO: This may need to be revisited, i.e., is a type assertion a
					// good solution here?
					// Note that if the CurrentQuestion.Answer is a slice, then we'll want
					// to use it as a bitmap.
					res := true
					switch vv := msg.Data.(type) {
					case float64:
						answer := game.CurrentQuestion.Answer.(uint16)
						// Compensate for the bit that we set for the multi-answer
						// multiple choice question.
						if answer>>15 == 1 {
							res = float64(answer) == (1<<15)+vv
						} else {
							res = float64(answer) == vv
						}
					case string:
						res = game.CurrentQuestion.Answer == vv
					}

					// Increment the field that we'll use to determine when every player
					// has responded.  At that point, we'll update the scoreboard.
					game.CurrentQuestion.Responses += 1

					// Message the player individually if the answer was correct (or not).
					b, err := json.Marshal(ServerMessage{
						Type: "player_message",
						Data: res,
					})
					if err != nil {
						fmt.Println("marshall error:", err)
					} else {
						// TODO: check if this actually sent?
						socket.Write(b)
						if !res {
							m, err := json.Marshal(ServerMessage{
								Type: "notify_player",
								Data: fmt.Sprintf("The correct answer is %f", game.CurrentQuestion.Answer),
							})
							if err != nil {
								fmt.Println("marshall error:", err)
							}
							if _, err := player.Socket.Write(m); err != nil {
								fmt.Println("socket write error:", err)
							}
						}
					}

					// Log the player's result.
					if res {
						_, err := game.UpdatePlayerScore(socket, game.CurrentQuestion.Weight)
						if err != nil {
							log.Fatalln(err)
						}
						fmt.Printf("%s correctly guessed %s, %d current points\n",
							player.Name,
							msg.Data,
							player.Score)
					} else {
						fmt.Printf("%s incorrectly guessed %s, %d current points\n",
							player.Name,
							msg.Data,
							player.Score)
					}

					// If everyone has answered, update everyone by updating the scoreboard.
					if len(game.Players) == game.CurrentQuestion.Responses {
						err = s.Publish(game, ServerMessage{
							Type: "update_scoreboard",
							Data: game.Players,
						})
						if err != nil {
							log.Fatalln(err)
						}
					}
				}
			}
		}
	}
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

// TODO: Use a CSV package for this?
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
		Choices:  choices,
		Weight:   weight,
	}

	if len(choices) > 0 {
		answers := strings.Split(l[1], ",")
		bitmap := makeBitmap(answers)
		// If there is more than one answer than it is a multiple
		// choice question with more than one right answer.
		// As such, we need to encode this into the bitmap, so the
		// UI can tell the difference between a multiple choice
		// question with only one right answer and one with more
		// than one.
		// A bit value of `10000000 00000000` will instruct the UI
		// to make checkbox options, while a bit value of
		// `00000000 00000000` will instruct it to make radio options.
		// weeeeeeeeeeeeeeeeeeeee
		if len(answers) > 1 {
			bitmap += 1 << 15
		}
		game.CurrentQuestion.Answer = bitmap
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
