package middleware

import (
	"context"
	"errors"
	"math/rand"
	"net/http"
	"time"
)

func apiKey(n int) string {
	alphanumeric := []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	key := make([]byte, n)
	for i := range key {
		key[i] = alphanumeric[rand.Intn(len(alphanumeric))]
	}
	return string(key)
}

func GenerateKey(name string, length int, seconds float64) APIKey {
	rand.Seed(time.Now().UTC().UnixNano())
	return APIKey{
		Key:         apiKey(length),
		TimeCreated: time.Now().UTC(),
		Expiration:  seconds,
		Expired:     false,
	}
}

type APIKey struct {
	Key         string
	TimeCreated time.Time
	Expiration  float64
	Expired     bool
}

type Authenticator struct {
	key     *APIKey
	handler http.Handler
}

// TODO: revisit this comment
//
// Called whenever a game is looked up [SocketServer.GetGame].
// It only checks for equality and not expiration because
// an already logged in player may still be sending requests
// to the socket server after the game has expired, which is
// legal.
// There needs to be a way to differentiate between a
// logged in user and one that is trying to log in after
// the game has expired, so breaking these token checks
// into there respective parts makes sense and accomplishes
// this goal.
// See [Game.CheckTokenExpiration] for more information.
func (a *Authenticator) checkTokenEquality(token string) error {
	if token == "" {
		return errors.New("No API Key")
	}
	if token != a.key.Key {
		return errors.New("Bad API key")
	}
	return nil
}

func (a *Authenticator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	keyHeader := r.Header.Get("X-TRIVIA-APIKEY")
	if keyHeader == "" && r.URL.Path == "/" || r.URL.Path == "/ws" {
		a.handler.ServeHTTP(w, r)
		return
	}
	if err := a.checkTokenEquality(keyHeader); err != nil {
		http.Error(w, "bad API key", http.StatusUnauthorized)
		return
	}
	authContext := context.WithValue(r.Context(), "apiKey", a.key)
	a.handler.ServeHTTP(w, r.WithContext(authContext))
}

func NewAuthenticator(key *APIKey, handler http.Handler) *Authenticator {
	return &Authenticator{key, handler}
}
