package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cms-go/internal/server"

	"github.com/joho/godotenv"
)

const addr = ":8080"

func main() {
	godotenv.Load()

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("cannot start: port %s is already in use: %v", addr, err)
	}

	srv := server.New()
	srv.Listener = listener

	go func() {
		if err := srv.Start(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			srv.Logger.Fatal(err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		srv.Logger.Fatal(err)
	}
}
