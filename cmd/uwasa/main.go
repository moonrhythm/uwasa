package main

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/acoshift/arpc/v2"
	"github.com/acoshift/configfile"
	"github.com/moonrhythm/httpmux"
	"github.com/moonrhythm/parapet"
	"github.com/moonrhythm/parapet/pkg/authn"

	"github.com/moonrhythm/uwasa"
)

const bearerPrefix = "bearer "

func main() {
	cfg := configfile.NewEnvReader()
	port := cfg.StringDefault("port", "8080")
	authToken := cfg.String("auth_token")
	bufferSize := cfg.IntDefault("buffer_size", 100)

	ctx := context.Background()
	s := uwasa.NewServer(ctx, bufferSize)
	am := arpc.New()

	mux := httpmux.New()
	mux.Handle("/", am.NotFoundHandler())
	mux.Handle("/publish", am.Handler(s.Publish))
	mux.Handle("/subscribe", am.Handler(s.Subscribe))

	srv := parapet.NewBackend()
	srv.Addr = ":" + port
	srv.Handler = mux

	srv.Use(authTokenMiddleware(authToken))

	log.Printf("server listening on %s", srv.Addr)

	err := srv.ListenAndServe()
	if err != nil {
		log.Fatalf("starting server error; %v", err)
	}
}

func authTokenMiddleware(authToken string) parapet.Middleware {
	if authToken == "" {
		return nil
	}

	return authn.Authenticator{
		Type: "bearer",
		Authenticate: func(r *http.Request) error {
			authHeader := r.Header.Get("Authorization")
			if len(authHeader) < len(bearerPrefix) {
				return uwasa.ErrForbidden
			}
			if !strings.EqualFold(authHeader[:len(bearerPrefix)], bearerPrefix) {
				return uwasa.ErrForbidden
			}
			if authHeader[len(bearerPrefix):] != authToken {
				return uwasa.ErrForbidden
			}
			return nil
		},
	}
}
