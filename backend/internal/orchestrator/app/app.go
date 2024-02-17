package app

import (
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/klef99/distributed-calculation-backend/internal/orchestrator/database"
	"github.com/klef99/distributed-calculation-backend/internal/orchestrator/services/distributor"
	"github.com/klef99/distributed-calculation-backend/internal/orchestrator/services/handlers"
	"github.com/klef99/distributed-calculation-backend/pkg/redis"
)

func Run() {
	if err := godotenv.Load(filepath.Join("/home/egor/code/git/distributed-calculation/.env")); err != nil {
		log.Fatal("No .env file found")
	}
	router := mux.NewRouter()
	conn := database.Connect()
	defer database.CloseConnection(conn)
	h := handlers.New(conn)
	RedisConn := redis.NewConnectionRedis()
	d := distributor.NewDistributor(RedisConn, conn)
	go d.NewOperations(2 * time.Second)
	go d.SendOperations(2 * time.Second)
	go d.GetOperationResult()
	go d.UpdateOperations(2 * time.Second)
	router.HandleFunc("/addExpression", h.AddExpression)
	router.HandleFunc("/getExpressionsList", h.GetExpressionsList)
	router.HandleFunc("/getExpressionByID", h.GetExpressionByID)
	err := http.ListenAndServe(":8080", router)
	if err != nil {
		log.Fatalln("There's an error with the server", err)
	}
}
