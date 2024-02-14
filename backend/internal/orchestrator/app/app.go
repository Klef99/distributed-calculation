package app

import (
	"log"
	"net/http"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/klef99/distributed-calculation-backend/internal/orchestrator/database"
	"github.com/klef99/distributed-calculation-backend/internal/orchestrator/services/handlers"
)

func Run() {
	if err := godotenv.Load(filepath.Join("/home/egor/code/git/distributed-calculation/.env")); err != nil {
		log.Fatal("No .env file found")
	}
	router := mux.NewRouter()
	conn := database.Connect()
	defer conn.CloseConnection()
	h := handlers.New(conn)
	router.HandleFunc("/addExpression", h.AddExpression)
	router.HandleFunc("/getExpressionsList", h.GetExpressionsList)
	err := http.ListenAndServe(":8080", router)
	if err != nil {
		log.Fatalln("There's an error with the server", err)
	}
}
