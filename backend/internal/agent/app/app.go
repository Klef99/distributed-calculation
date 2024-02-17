package app

import (
	"log"
	"net/http"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/klef99/distributed-calculation-backend/internal/agent/services/handlers"
	"github.com/klef99/distributed-calculation-backend/internal/agent/services/pool"
	"github.com/klef99/distributed-calculation-backend/internal/agent/services/worker"
	"github.com/klef99/distributed-calculation-backend/pkg/redis"
)

func Run() {
	if err := godotenv.Load(filepath.Join("/home/egor/code/git/distributed-calculation/.env")); err != nil {
		log.Fatal("No .env file found")
	}
	router := mux.NewRouter()
	p := pool.New(10)
	defer p.Shutdown()
	conn := redis.NewConnectionRedis()
	defer redis.CloseConnectionRedis(conn)
	w := worker.NewWorker(conn, p)
	go w.SetOperationsToCalc()
	go w.SendOperationResults()
	router.HandleFunc("/hearthbeat", handlers.HearthbeatHandler)
	err := http.ListenAndServe(":8090", router)
	if err != nil {
		log.Fatalln("There's an error with the server", err)
	}
}
