[View the documentation](https://pkg.go.dev/github.com/btoll/trivial)

## Endpoints

- [`/kill`](https://pkg.go.dev/github.com/btoll/trivial#SocketServer.KillHandler)
- [`/message`](https://pkg.go.dev/github.com/btoll/trivial#SocketServer.MessageHandler)
- [`/notify`](https://pkg.go.dev/github.com/btoll/trivial#SocketServer.NotifyHandler)
- [`/query`](https://pkg.go.dev/github.com/btoll/trivial#SocketServer.QueryHandler)
- [`/reset`](https://pkg.go.dev/github.com/btoll/trivial#SocketServer.ResetHandler)
- [`/scoreboard`](https://pkg.go.dev/github.com/btoll/trivial#SocketServer.ScoreboardHandler)

## Serve the docs

```
$ godoc -play
```

This will use the default port of `6060`.  Then, point your browser to:

`http://localhost:6060/pkg/github.com/btoll/trivial/`

> This also enables the Playground.

## Test the `/query` Endpoint

To send a single question to the game players, you can run the following query (of course, replace the `URL` with your own) :

```
$ curl -XGET -H "X-TRIVIA-APIKEY: GwCcn6VZntwnI7qp1kly" \
    --data "What year did the Beatles play Budokan?,B,50,1965,1968,1970" \
    127.0.0.1:3000/query
```

An easier way would be to concatenate all of the `csv` files together and then create a variable which will be incremented to pull the questions from the file line-by-line.

```
$ cat *.csv > game.csv
```

Set the variable to increment:

```
$ i=0
```

Push questions through to the game players by issuing a single chained command that first updates the variable and then uses it to pick the respective line from the `game.csv` file:

```
$ i=$((i+1)) && curl -XGET -H "X-TRIVIA-APIKEY: GwCcn6VZntwnI7qp1kly" \
    --data "$(awk 'NR=='$i'' game.csv)" \
    127.0.0.1:3000/query
```

## Binary Search Implementations

- [returns `bool`](https://go.dev/play/p/ch11-8OM-HT)
- [returns index `int`](https://go.dev/play/p/bVW_8iNdnid)

## References

- [Golang `websocket` Documentation](https://pkg.go.dev/golang.org/x/net/websocket)
- [How To Build A Chat And Data Feed With WebSockets In Golang?](https://www.youtube.com/watch?v=JuUAEYLkGbM)
- [Is it possible to have nested templates in Go using the standard library?](https://stackoverflow.com/questions/11467731/is-it-possible-to-have-nested-templates-in-go-using-the-standard-library)
- [Nested `template` Gist](https://gist.github.com/joyrexus/ff9be7a1c3769a84360f)
- [Serving Static Sites with Go](https://www.alexedwards.net/blog/serving-static-sites-with-go)
- [Using Nested Templates in Go for Efficient Web Development](https://levelup.gitconnected.com/using-go-templates-for-effective-web-development-f7df10b0e4a0)

## License

[GPLv3](COPYING)

## Author

Benjamin Toll

