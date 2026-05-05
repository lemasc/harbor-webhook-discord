package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, using environment variables")
	}

	discordURL := os.Getenv("DISCORD_WEBHOOK_URL")
	if discordURL == "" {
		log.Fatal("DISCORD_WEBHOOK_URL is required")
	}

	host := os.Getenv("HOST")
	if host == "" {
		host = "127.0.0.1"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := host + ":" + port
	handler := NewWebhookHandler(discordURL)
	http.HandleFunc("/webhook", handler.HandleWebhook)

	fmt.Printf("listening on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
