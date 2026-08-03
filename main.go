package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cms-go/internal/server"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()
	srv := server.New()

	go func() {
		if err := srv.Start(":8080"); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
