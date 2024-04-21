package worker

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/klef99/distributed-calculation-backend/internal/agent/services/pool"
	"github.com/klef99/distributed-calculation-backend/pkg/calc"
	"github.com/klef99/distributed-calculation-backend/pkg/redis"
)

type Worker struct {
	conn *redis.ConnectionRedis
	pool *pool.Pool
}

func NewWorker(conn *redis.ConnectionRedis, pool *pool.Pool) *Worker {
	return &Worker{conn: conn, pool: pool}
}

func (w *Worker) SetOperationsToCalc() {
	pubsub := w.conn.GetSubscribe("operations")
	defer pubsub.Close()
	for {
		_, err := pubsub.ReceiveMessage(context.Background())
		if err != nil {
			panic(err)
		}
		var operation calc.Operation
		data, err := w.conn.GetOperationToCalc()
		if err != nil {
			panic(err)
		}
		if data == "nil" {
			continue
		}
		json.Unmarshal([]byte(data), &operation)
		timeouts, err := w.conn.GetOperationsTimeouts()
		if err != nil {
			panic(err)
		}
		w.pool.Run(operation, timeouts)
	}
}

func (w *Worker) SendOperationResults() {
	pubsub := w.conn.GetSubscribe("results")
	defer pubsub.Close()
	for res := range w.pool.Results {
		err := w.conn.SendOperationResult(res)
		if err != nil {
			slog.Info(err.Error())
		}
	}
}
