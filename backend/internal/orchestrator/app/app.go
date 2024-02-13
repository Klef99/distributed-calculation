package app

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/klef99/distributed-calculation-backend/internal/orchestrator/services/handlers"
)

func Run() {
	router := mux.NewRouter()
	router.HandleFunc("/addExpression", handlers.AddExpression)
	err := http.ListenAndServe(":8080", router)
	if err != nil {
		log.Fatalln("There's an error with the server", err)
	}
}
