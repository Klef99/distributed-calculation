package database

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/klef99/distributed-calculation-backend/pkg/calc"
)

type Connection struct {
	conn *pgxpool.Pool
}

func Connect() *Connection {
	connInfo := fmt.Sprintf("host=%s port=%s user=%s "+
		"password=%s dbname=%s sslmode=disable",
		"localhost", os.Getenv("POSTGRES_PORT"), os.Getenv("POSTGRES_USER"), os.Getenv("POSTGRES_PASSWORD"), os.Getenv("POSTGRES_DB"))
	dbpool, err := pgxpool.New(context.Background(), connInfo)
	if err != nil {
		log.Fatal(err)
	}
	err = dbpool.Ping(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	slog.Info("Successfully connected to db.")
	return &Connection{conn: dbpool}
}

func CloseConnection(c *Connection) {
	defer c.conn.Close()
}

func (c *Connection) InsertExpression(ctx context.Context, id, expr string) error {
	query := `INSERT INTO expressions(expressionid, expression, status) VALUES (@expressionId, @expression, @status) returning expressionid`
	args := pgx.NamedArgs{
		"expressionId": id,
		"expression":   expr,
		"status":       0,
	}
	_, err := c.conn.Exec(ctx, query, args)
	if err != nil {
		return fmt.Errorf("unable to insert row: %w", err)
	}
	return nil
}

func (c *Connection) GetExpressions(ctx context.Context) ([]struct {
	Uuid   string `json:"expressionid"`
	Expr   string `json:"expression"`
	Status int    `json:"status"`
}, error) {
	query := `SELECT expressionid, expression, status FROM expressions`
	rows, err := c.conn.Query(ctx, query)
	if err != nil {
		return []struct {
			Uuid   string `json:"expressionid"`
			Expr   string `json:"expression"`
			Status int    `json:"status"`
		}{}, fmt.Errorf("unable to query expressions: %w", err)
	}
	defer rows.Close()
	exprs := []struct {
		Uuid   string `json:"expressionid"`
		Expr   string `json:"expression"`
		Status int    `json:"status"`
	}{}
	for rows.Next() {
		expr := struct {
			Uuid   string `json:"expressionid"`
			Expr   string `json:"expression"`
			Status int    `json:"status"`
		}{}
		err := rows.Scan(&expr.Uuid, &expr.Expr, &expr.Status)
		if err != nil {
			return []struct {
				Uuid   string `json:"expressionid"`
				Expr   string `json:"expression"`
				Status int    `json:"status"`
			}{}, fmt.Errorf("unable to scan row: %w", err)
		}
		exprs = append(exprs, expr)
	}

	return exprs, nil
}

func (c *Connection) GetExpressionByID(ctx context.Context, expressionid string) (interface{}, error) {
	// ctxWithT, cancel := context.WithTimeout(ctx, time.Second*2)
	// defer cancel()
	query := `SELECT result FROM expressions where expressionid = @expressionId`
	args := pgx.NamedArgs{
		"expressionId": expressionid,
	}
	rows, err := c.conn.Query(ctx, query, args)
	if err != nil {
		return "", fmt.Errorf("unable to query expression: %w", err)
	}
	defer rows.Close()
	var result interface{}
	for rows.Next() {
		err := rows.Scan(&result)
		if err != nil {
			return "", fmt.Errorf("unable to scan row: %w", err)
		}
	}
	return result, nil
}

func (c *Connection) GetNotPartitionExpressions(ctx context.Context) ([][]string, error) {
	// ctxWithT, cancel := context.WithTimeout(ctx, time.Second*2)
	// defer cancel()
	query := `SELECT expressionid, expression FROM expressions where status = 0`
	rows, err := c.conn.Query(ctx, query)
	if err != nil {
		return [][]string{}, fmt.Errorf("unable to query expressions: %w", err)
	}
	defer rows.Close()
	var result [][]string
	for rows.Next() {
		var res = make([]string, 2)
		err := rows.Scan(&res[0], &res[1])
		if err != nil {
			return [][]string{}, fmt.Errorf("unable to scan row: %w", err)
		}
		result = append(result, res)
	}
	return result, nil
}

func (c *Connection) BulkInsertOperations(ctx context.Context, tasks []calc.Operation) error {
	query := `INSERT INTO operations (operationid, operator, v1, v2, expressionid, parentid, "left", status) VALUES (@operationid, @operator, @v1, @v2, @expressionid, @parentid, @left,@status)`

	batch := &pgx.Batch{}
	for _, task := range tasks {
		args := pgx.NamedArgs{
			"expressionid": task.ExpressionID,
			"operator":     task.Operator,
			"v1":           task.V1,
			"v2":           task.V2,
			"operationid":  task.OperationID,
			"parentid":     task.ParentID,
			"left":         task.Left,
			"status":       task.Status,
		}
		batch.Queue(query, args)
	}
	results := c.conn.SendBatch(ctx, batch)
	defer results.Close()
	for _, task := range tasks {
		_, err := results.Exec()
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
				slog.Info("operation %s already exists", task.OperationID)
				continue
			}
			slog.Info(fmt.Sprint(task.ExpressionID, task.OperationID, task.ParentID))
			return fmt.Errorf("unable to insert row: %w", err)
		}
	}
	return results.Close()
}

func (c *Connection) ChangeExpressionStatus(ctx context.Context, expressionid string, status int) error {
	query := `UPDATE expressions SET status = @status WHERE expressions.expressionid = @expressionId`
	args := pgx.NamedArgs{
		"expressionId": expressionid,
		"status":       status,
	}
	_, err := c.conn.Exec(ctx, query, args)
	if err != nil {
		return fmt.Errorf("unable to update row: %w", err)
	}
	slog.Info("Changed expression (%s) status to %d", expressionid, status)
	return nil
}

func (c *Connection) GetExpressionToExecution(ctx context.Context) ([]calc.Operation, error) {
	query := `SELECT operationid, operator, v1, v2, expressionid, parentid, "left" FROM operations where v1 IS NOT NULL and v2 is not null and status = 0`
	rows, err := c.conn.Query(ctx, query)
	if err != nil {
		return []calc.Operation{}, fmt.Errorf("unable to query operations: %w", err)
	}
	defer rows.Close()
	result := []calc.Operation{}
	for rows.Next() {
		var res = calc.Operation{}
		err := rows.Scan(&res.OperationID, &res.Operator, &res.V1, &res.V2, &res.ExpressionID, &res.ParentID, &res.Left)
		if err != nil {
			return []calc.Operation{}, fmt.Errorf("unable to scan row: %w", err)
		}
		result = append(result, res)
	}
	return result, nil
}

func (c *Connection) BulkChangeStatusOperations(ctx context.Context, status int, operations []calc.Operation) error {
	query := `UPDATE operations SET status = @status where operationid = @operationid`
	batch := &pgx.Batch{}
	for _, task := range operations {
		args := pgx.NamedArgs{
			"operationid": task.OperationID,
			"status":      status,
		}
		batch.Queue(query, args)
	}
	results := c.conn.SendBatch(ctx, batch)
	defer results.Close()
	for _, task := range operations {
		_, err := results.Exec()
		if err != nil {
			slog.Info(fmt.Sprint(task.ExpressionID, task.OperationID, task.ParentID))
			return fmt.Errorf("unable to insert row: %w", err)
		}
	}
	return results.Close()
}

func (c *Connection) SetOperationResult(ctx context.Context, operationid string, result float64) error {
	query := `UPDATE operations SET result = @result where operationid = @operationid`
	args := pgx.NamedArgs{
		"operationid": operationid,
		"result":      result,
	}
	_, err := c.conn.Exec(ctx, query, args)
	if err != nil {
		return fmt.Errorf("unable to update row: %w", err)
	}
	slog.Info("Get operation (%s) result: %d", operationid, result)
	return nil
}
