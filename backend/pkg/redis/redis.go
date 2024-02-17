package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

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

// func (cr *ConnectionRedis) SetOperationTimeouts(timeouts map[string]time.Duration) error {

// 	ctx := context.Background()
// 	for k, v := range
// 	_, err := cr.conn.HSet(ctx, "operationTimeouts", key, value).Result()
// 	if err != nil {
// 		return fmt.Errorf("error while doing HSET command in gredis : %v", err)
// 	}

// 	return err
// }

func (cr *ConnectionRedis) SendOperationToRedis(operations []calc.Operation) error {
	for _, operation := range operations {
		p, err := json.Marshal(operation)
		if err != nil {
			return err
		}
		err = cr.conn.Publish(context.Background(), "operations", p).Err()
		if err != nil {
			return err
		}
	}
	return nil
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
