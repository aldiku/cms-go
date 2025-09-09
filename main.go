package main

import (
	"cms-go/internal/server"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()
	srv := server.New()
	srv.Logger.Fatal(srv.Start(":8080"))
}
