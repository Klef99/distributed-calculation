package main

import (
	"github.com/klef99/distributed-calculation-backend/internal/agent/app"
)

func main() {
	// err := godotenv.Load("../../../.env")
	// if err != nil {
	// 	log.Fatal("Error loading .env file")
	// }
	app.Run()
}
