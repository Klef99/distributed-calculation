package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/klef99/distributed-calculation-backend/pkg/calc"
	"github.com/redis/go-redis/v9"
)

type ConnectionRedis struct {
	conn *redis.Client
}

func NewConnectionRedis() *ConnectionRedis {
	return &ConnectionRedis{conn: redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", os.Getenv("REDIS_ADDRESS"), os.Getenv("REDIS_PORT")),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0, // use default DB
	})}
}

func CloseConnectionRedis(cr *ConnectionRedis) {
	defer cr.conn.Close()
}

func (cr *ConnectionRedis) BulkSetOperationsTimeouts(timeouts map[string]int, userid int) error {
	ctx := context.Background()
	for key, value := range timeouts {
		flag := true
		for _, v := range []string{"+", "-", "/", "*"} {
			if key == v {
				flag = false
			}
		}
		if flag {
			continue
		}
		err := cr.conn.HSet(ctx, "operationTimeouts_"+strconv.Itoa(userid), key, value*int(time.Second.Nanoseconds())).Err()
		if err != nil {
			return fmt.Errorf("error while doing HSET command in gredis : %v", err)
		}
	}
	return nil
}
func (cr *ConnectionRedis) GetOperationsTimeouts(userid int) (map[string]time.Duration, error) {
	ctx := context.Background()
	cmd := cr.conn.HGetAll(ctx, "operationTimeouts_"+strconv.Itoa(userid))
	if cmd.Err() != nil {
		return map[string]time.Duration{}, cmd.Err()
	}
	timeouts := make(map[string]time.Duration, 0)
	for k, v := range cmd.Val() {
		t, err := time.ParseDuration(v + "ns")
		if err != nil {
			t = time.Second * 10
		}
		timeouts[k] = t
	}
	return timeouts, nil
}

func (cr *ConnectionRedis) SendOperationToRedis(operations []calc.Operation, userids []int) error {
	for i, operation := range operations {
		p, err := json.Marshal(operation)
		if err != nil {
			return err
		}
		err = cr.conn.RPush(context.Background(), "operations_lists", p).Err()
		if err != nil {
			return err
		}
		err = cr.conn.Publish(context.Background(), "operations", userids[i]).Err()
		if err != nil {
			return err
		}
	}
	return nil
}

func (cr *ConnectionRedis) GetOperationToCalc() (string, error) {
	oper := cr.conn.RPop(context.Background(), "operations_lists")
	if oper.Err() == redis.Nil {
		return "nil", nil
	}
	if oper.Err() != nil {
		return "", oper.Err()
	}
	return oper.Result()

}

func (cr *ConnectionRedis) GetSubscribe(channleName string) *redis.PubSub {
	return cr.conn.Subscribe(context.Background(), channleName)
}

func (cr *ConnectionRedis) SendOperationResult(operation struct {
	OperationID string
	Res         float64
}) error {
	p, err := json.Marshal(operation)
	if err != nil {
		return err
	}
	err = cr.conn.Publish(context.Background(), "results", p).Err()
	if err != nil {
		return err
	}
	return nil
}

func (cr *ConnectionRedis) SetWorkerStatus(ctx context.Context, worker string, taskCount int) error {
	err := cr.conn.HSet(ctx, "workers", worker, time.Now().Format(time.RFC3339Nano)).Err()
	if err != nil {
		return err
	}
	err = cr.conn.HSet(ctx, "workersTaskCount", worker, taskCount).Err()
	if err != nil {
		return err
	}
	return nil
}

func (cr *ConnectionRedis) GetWorkersStatus(ctx context.Context) ([]struct {
	WorkerName string `json:"workerName"`
	Status     string `json:"status"`
	TaskCount  string `json:"taskCount"`
}, error) {
	workerTime := cr.conn.HGetAll(ctx, "workers")
	if workerTime.Err() != nil {
		return []struct {
			WorkerName string `json:"workerName"`
			Status     string `json:"status"`
			TaskCount  string `json:"taskCount"`
		}{}, workerTime.Err()
	}
	workerTaskCount := cr.conn.HGetAll(ctx, "workersTaskCount")
	if workerTaskCount.Err() != nil {
		return []struct {
			WorkerName string `json:"workerName"`
			Status     string `json:"status"`
			TaskCount  string `json:"taskCount"`
		}{}, workerTime.Err()
	}
	workerStatus := make([]struct {
		WorkerName string `json:"workerName"`
		Status     string `json:"status"`
		TaskCount  string `json:"taskCount"`
	}, 0)
	for k, v := range workerTime.Val() {
		status := "OK"
		t, err := time.Parse(time.RFC3339Nano, v)
		if err != nil {
			status = "NO RESPONSE"
		}
		diff := time.Since(t)
		if diff > time.Minute {
			status = "NO RESPONSE"
		}
		tmp := struct {
			WorkerName string `json:"workerName"`
			Status     string `json:"status"`
			TaskCount  string `json:"taskCount"`
		}{WorkerName: k, Status: status, TaskCount: workerTaskCount.Val()[k]}
		workerStatus = append(workerStatus, tmp)
	}
	return workerStatus, nil
}
