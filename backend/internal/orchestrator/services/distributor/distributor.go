package distributor

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
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
		slog.Info("Starting seporation of expression")
		wg := &sync.WaitGroup{}
		for _, row := range rows { // row[0] - expressionId, row[1] - expression
			wg.Add(1)
			go func(row []string) {
				defer wg.Done()
				tasks, err := calc.TransformExpressionToStack(row[0], row[1])
				if err != nil {
					slog.Warn(err.Error())
					err = d.PostgresConn.ChangeExpressionStatus(context.Background(), row[0], -1)
					if err != nil {
						slog.Warn(err.Error())
					}
					return
				}
				err = d.PostgresConn.BulkInsertOperations(context.Background(), tasks)
				if err != nil {
					slog.Warn(err.Error())
					return
				}
				err = d.PostgresConn.ChangeExpressionStatus(context.Background(), row[0], 1)
				if err != nil {
					slog.Warn(err.Error())
				}
			}(row)
		}
		wg.Wait()
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
		workers, err := d.RedisConn.GetWorkersStatus(context.Background())
		if err != nil {
			slog.Warn("Avaliable workers not found.")
		}
		flag := false
		for _, w := range workers {
			if w.Status == "OK" {
				flag = true
				break
			}
		}
		if !flag {
			slog.Warn("Avaliable workers not found.")
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

func (d *Distributor) UpdateOperations(tick time.Duration) {
	ticker := time.NewTicker(tick)
	for range ticker.C {
		operations, err := d.PostgresConn.GetComplitedOperation(context.Background())
		if err != nil {
			slog.Warn(err.Error())
			continue
		}
		opList := make([]struct {
			Operationid string
			Parentid    string
			Res         float64
			Left        bool
		}, 0)
		var notFinalOperations = make([]calc.Operation, 0)
		for _, operation := range operations {
			if operation.ExpressionID == operation.ParentID {
				err := d.PostgresConn.SetExpressionResult(context.Background(), operation.ExpressionID, operation.Result.(float64))
				if err != nil {
					slog.Warn(err.Error())
					continue
				}
				err = d.PostgresConn.ChangeExpressionStatus(context.Background(), operation.ExpressionID, 2)
				if err != nil {
					slog.Warn(err.Error())
					continue
				}
				err = d.PostgresConn.ChangeOperationStatus(context.Background(), operation.OperationID, 2)
				if err != nil {
					slog.Warn(err.Error())
					continue
				}
				continue
			}
			op := struct {
				Operationid string
				Parentid    string
				Res         float64
				Left        bool
			}{Operationid: operation.OperationID, Left: operation.Left, Parentid: operation.ParentID, Res: operation.Result.(float64)}
			opList = append(opList, op)
			notFinalOperations = append(notFinalOperations, operation)
		}
		err = d.PostgresConn.SetOperationResultToParent(context.Background(), opList)
		if err != nil {
			slog.Warn(err.Error())
			continue
		}
		err = d.PostgresConn.BulkChangeStatusOperations(context.Background(), 2, notFinalOperations)
		if err != nil {
			slog.Warn(err.Error())
			continue
		}
	}
}

func (d *Distributor) RestoreStuckedOperation(tick time.Duration) {
	ticker := time.NewTicker(tick)
	for range ticker.C {
		timeouts, err := d.RedisConn.GetOperationsTimeouts()
		if err != nil {
			slog.Warn(err.Error())
			continue
		}
		maxTimeout := time.Nanosecond
		for _, v := range timeouts {
			maxTimeout = max(maxTimeout, v)
		}
		err = d.PostgresConn.UpdateStuckedOperations(context.Background(), maxTimeout)
		if err != nil {
			slog.Warn(err.Error())
			continue
		}
	}
}
