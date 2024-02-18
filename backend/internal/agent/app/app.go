package app

import (
	"log/slog"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/klef99/distributed-calculation-backend/internal/agent/services/pool"
	"github.com/klef99/distributed-calculation-backend/internal/agent/services/worker"
	"github.com/klef99/distributed-calculation-backend/pkg/redis"
)

func Run() {
	max, err := strconv.Atoi(os.Getenv("MAX_GOROUTINE_PER_AGENT"))
	if err != nil {
		slog.Info("not correct .env variable: MAX_GOROUTINE_PER_AGENT")
		max = 10
	}
	p := pool.New(max)
	defer p.Shutdown()
	conn := redis.NewConnectionRedis()
	defer redis.CloseConnectionRedis(conn)
	w := worker.NewWorker(conn, p)
	go w.SetOperationsToCalc()
	go w.SendOperationResults()
	go p.SendHearthbeat(os.Getenv("WORKER_NAME"), time.Second*1)
	wg := sync.WaitGroup{}
	wg.Add(1)
	wg.Wait()
}
