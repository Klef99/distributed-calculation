package distributor

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/klef99/distributed-calculation-backend/internal/orchestrator/database"
	"github.com/klef99/distributed-calculation-backend/pkg/calc"
	"github.com/klef99/distributed-calculation-backend/pkg/redis"
)

type Distributor struct {
	RedisConn    *redis.ConnectionRedis
	PostgresConn *database.Connection
}

func NewDistributor(RedisConn *redis.ConnectionRedis, PostgresConn *database.Connection) *Distributor {
	return &Distributor{RedisConn: RedisConn, PostgresConn: PostgresConn}
}

func (d *Distributor) NewOperations(tick time.Duration) {
	ticker := time.NewTicker(tick)
	for range ticker.C {
		rows, err := d.PostgresConn.GetNotPartitionExpressions(context.Background())
		if err != nil {
			slog.Error(err.Error())
			continue
		}
		if len(rows) == 0 {
			slog.Info("No rows to seporating")
			continue
		}
		slog.Info("Starting seportion of expression")
		for _, row := range rows { // row[0] - expressionId, row[1] - expression
			tasks := calc.TransformExpressionToStack(row[0], row[1])
			err := d.PostgresConn.BulkInsertOperations(context.Background(), tasks)
			if err != nil {
				slog.Warn(err.Error())
				continue
			}
			err = d.PostgresConn.ChangeExpressionStatus(context.Background(), row[0], 1)
			if err != nil {
				slog.Warn(err.Error())
			}
		}
	}
}

func (d *Distributor) SendOperations(tick time.Duration) {
	ticker := time.NewTicker(tick)
	for range ticker.C {
		avalibleOperations, err := d.PostgresConn.GetExpressionToExecution(context.Background())
		if err != nil {
			slog.Warn(err.Error())
			continue
		}
		if len(avalibleOperations) == 0 {
			continue
		}
		err = d.RedisConn.SendOperationToRedis(avalibleOperations)
		if err != nil {
			slog.Warn(err.Error())
			continue
		}
		err = d.PostgresConn.BulkChangeStatusOperations(context.Background(), 1, avalibleOperations)
		if err != nil {
			slog.Warn(err.Error())
			continue
		}
	}
}

func (d *Distributor) GetOperationResult() {
	pubsub := d.RedisConn.GetSubscribe("results")
	defer pubsub.Close()
	for {
		msg, err := pubsub.ReceiveMessage(context.Background())
		if err != nil {
			panic(err)
		}
		var operation struct {
			OperationID string
			Res         float64
		}
		json.Unmarshal([]byte(msg.Payload), &operation)
		err = d.PostgresConn.SetOperationResult(context.Background(), operation.OperationID, operation.Res)
		if err != nil {
			slog.Warn(err.Error())
		}
	}
}

func (d *Distributor) UpdateOperations(tick time.Duration){
	ticker := time.NewTicker(tick)
	for range ticker.C{
		
	}
}