package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"

	"github.com/btoll/trivial/src/server"
)

var (
	wssURL          = flag.String("wss", "wss://127.0.0.1:3000", "URL of game websocket server")
	hostURL         = flag.String("host", "https://127.0.0.1:3000", "URL of game host server")
	gameName        = flag.String("game", "default", "Name of game")
	generateCert    = flag.Bool("generateCert", false, "Generate a new TLS certificate")
	tokenExpiration = flag.Float64("tokenExpiration", 3600, "Token expiration (in seconds)")
)

func parseURL(s string) server.Socket {
	parsedUrl, err := url.Parse(s)
	if err != nil {
		log.Fatalln("server url could not be parsed")
	}
	port, err := strconv.Atoi(parsedUrl.Port())
	if err != nil {
		log.Fatalln("port could not be parsed")
	}
	return server.Socket{
		Protocol: parsedUrl.Scheme,
		Domain:   parsedUrl.Hostname(),
		Port:     port,
	}
}

func bound(n int) string {
	return strings.Repeat("-", n)
}

func main() {
	flag.Parse()

	wssSock := parseURL(*wssURL)
	hostSock := parseURL(*hostURL)
	socketServer := server.URL{
		Sock: wssSock,
		Path: "ws",
	}

	sockserv := server.NewSocketServer(socketServer)
	fmt.Printf("%s\ncreated new websocket server `%s`\n",
		bound(75),
		socketServer)

	if *generateCert {
		server.GenerateCert(server.TLSCert{
			EcdsaCurve: "P384",
			Host:       fmt.Sprintf("%s,%s", "127.0.0.1", hostSock.Domain),
			IsCA:       true,
			RsaBits:    3072,
		})
		fmt.Printf("generated new TLS certificate for domains `%s` and `%s`\n", "127.0.0.1", hostSock.Domain)
	}

	game := server.NewGame(*gameName, *tokenExpiration)
	fmt.Printf("registered game `%s` with key `%s` on host `%s`\n%s\n",
		game.Name,
		game.Key.Key,
		hostSock,
		bound(75))
	sockserv.RegisterAndStartGame(game)
}
