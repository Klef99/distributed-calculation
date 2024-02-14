package database

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

func (c *Connection) CloseConnection() {
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
		}{}, fmt.Errorf("unable to query users: %w", err)
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
