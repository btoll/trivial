# `trivial`

`trivial` is an API server for trivia games.  It sets up a new trivia game and allows for multiple players to join using a time-sensitive token that is generated when the new game is started.

It is easy to create a new game.  Here are the following steps:

1. Create a new socket server.  This will define the host location of where the game is hosted (which you send to each player) and expose the APIs which the game will use throughout the session.  In addition, it exposes a `Games` map to add the new game to, which allows for multiple games to be hosted on the same socket server.

1. Optionally generate a `TLS` certificate.  The game will expect to find the `cert.pem` and `key.pem` files in the same directory as the `trivial` binary, so you can "bring your own" or optionally have the [`trivial`] API server generate them for you.

1. Create and register the new game.

1. Start the game.

The `trivial` game will take care of most of this for you.  For example, once the binary is placed on the publicly-accessible remote server, a simple command such as the following is all that is needed to start a game:

```bash
$ ./trivial -generateCert -wss wss://167.114.97.28:3000
---------------------------------------------------------------------------
created new websocket server `wss://167.114.97.28:3000/ws`
generated new TLS certificate for domains `127.0.0.1` and `127.0.0.1`
registered game `default` with key `bZu5SaAQ5d3EEwz1bkEp` on host `https://127.0.0.1:3000`
---------------------------------------------------------------------------
```

- If you already have created your cert, simply omit the `-generateCert` flag.
- The web socket `URL` needs to be the same as the server `IP` address.
- The generated private key (`bZu5SaAQ5d3EEwz1bkEp` in this example) should be distributed to all of the game players.  This is a time-sensitive token that will only allow a player to successfully login up to one hour from the time of the token creation.
- Distribute the `URL` of the game server to all of the players (i.e., `https://167.114.97.28:3000`).  Once there, they can choose a username and enter the private key (`bZu5SaAQ5d3EEwz1bkEp`).  This will allow them entry to the game.

<!--## Testing the `/query` Endpoint-->

## Controlling the Game

The question and the answer(s) are delimited by the pipe (`|`) symbol.  Here is a breakdown of the format:

```
Question?|Number of points given for a correct answer.|The correct answer.|Multiple possible answers, each separated by a pipe symbol (`|`)
```

For example:

```
This teenage guitar player once sat in for an ill Richie Blackmore at a Deep Purple show.|50|4|Tommy Bolin|Billy Gibbons|Joe Walsh|Christopher Cross|Earl Slick|Peter Frampton
```

The `4` in the third field indicates that the correct answer to the above question is `Christopher Cross`.

Also, a question can have more than one correct answer.  Simply separate each correct answer by a comma (`,`).

For instance, the following question has four correct answers:

```
Name the Beatles?|50|1,2,3,5|John|Paul|George|Tony|Ringo
```

Again, the third field indicates the corrrect answers.

> For questions with more than one correct answer, the `html` will be a `checkbox` component, rather than the default `radio` component.
>
> This also serves as a visual clue as to the question's intent.

Currently, game control is facilitated on the command line using `curl`.  Here is an example:

```bash
$ curl -XGET -H "X-TRIVIA-APIKEY: bZu5SaAQ5d3EEwz1bkEp" \
    --data "What year did the Beatles play Budokan?|50|2|1965|1968|1970" \
    127.0.0.1:3000/query
```

> Note that the `X-TRIVIA-APIKEY` header expects the private key that was generated when the server was started.  Refer to the logs above.

This will send the question to the game players.  If you're controlling the game from a remote machine, replace the `loopback` address with the `IP` address or domain of the remote server.

An easier way to send questions to the players would be to concatenate all of the `csv` files together which contain the questions in the format specified above and then create a variable which will be incremented to pull the questions from the file line-by-line:

```bash
$ cat *.csv > game.csv
```

Then, set the variable to increment:

```bash
$ i=0
```

Now, push questions through to the game players by issuing a single chained command that first updates the variable and then uses it to pick the respective line from the `game.csv` file:

```bash
$ i=$((i+1)) && curl -XGET -H "X-TRIVIA-APIKEY: bZu5SaAQ5d3EEwz1bkEp" \
    --data "$(awk 'NR=='$i'' game.csv)" \
    127.0.0.1:3000/query
```

> If using a self-signed `TLS` certificate, pass the `-k` or `--insecure` switch so `curl` will disable strict certificate checking.
>
> ```bash
> $ curl --insecure -H "X-TRIVIA-APIKEY: bZu5SaAQ5d3EEwz1bkEp" \
>     --data "$(awk 'NR=='$i'' questions/roman_history.csv)" \
>     127.0.0.1:3000/query
> ```

## Endpoints

- [`/kill`](https://pkg.go.dev/github.com/btoll/trivial#SocketServer.KillHandler)
- [`/message`](https://pkg.go.dev/github.com/btoll/trivial#SocketServer.MessageHandler)
- [`/notify`](https://pkg.go.dev/github.com/btoll/trivial#SocketServer.NotifyHandler)
- [`/query`](https://pkg.go.dev/github.com/btoll/trivial#SocketServer.QueryHandler)
- [`/reset`](https://pkg.go.dev/github.com/btoll/trivial#SocketServer.ResetHandler)
- [`/scoreboard`](https://pkg.go.dev/github.com/btoll/trivial#SocketServer.ScoreboardHandler)
- [`/update_score`](https://pkg.go.dev/github.com/btoll/trivial#SocketServer.UpdateScoreHandler)

## Serve the docs

```
$ godoc -play
```

This will use the default port of `6060`.  Then, point your browser to:

`http://localhost:6060/pkg/github.com/btoll/trivial/`

> This also enables the Playground.

## Troubleshooting

If the server that is running the game has an older version of `glibc`, you may encounter the following error when starting the game:

```bash
$ ./trivial
./trivial: /lib/x86_64-linux-gnu/libc.so.6: version `GLIBC_2.32' not found (required by ./trivial)
./trivial: /lib/x86_64-linux-gnu/libc.so.6: version `GLIBC_2.34' not found (required by ./trivial)
```

Here is one way "fix" that error:

```bash
$ CGO_ENABLED=0 go build
```

Or, install a previous version of Go.

## Binary Search Implementations

- [returns `bool`](https://go.dev/play/p/ch11-8OM-HT)
- [returns index `int`](https://go.dev/play/p/bVW_8iNdnid)

## References

- [View the documentation](https://pkg.go.dev/github.com/btoll/trivial)
- [Golang `websocket` Documentation](https://pkg.go.dev/golang.org/x/net/websocket)
- [How To Build A Chat And Data Feed With WebSockets In Golang?](https://www.youtube.com/watch?v=JuUAEYLkGbM)
- [Is it possible to have nested templates in Go using the standard library?](https://stackoverflow.com/questions/11467731/is-it-possible-to-have-nested-templates-in-go-using-the-standard-library)
- [Nested `template` Gist](https://gist.github.com/joyrexus/ff9be7a1c3769a84360f)
- [Serving Static Sites with Go](https://www.alexedwards.net/blog/serving-static-sites-with-go)
- [Using Nested Templates in Go for Efficient Web Development](https://levelup.gitconnected.com/using-go-templates-for-effective-web-development-f7df10b0e4a0)

## License

[GPLv3](COPYING)

## Author

[Benjamin Toll](https://benjamintoll.com)

