package database

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/klef99/distributed-calculation-backend/pkg/calc"
	"golang.org/x/crypto/bcrypt"
)

type Connection struct {
	conn *pgxpool.Pool
}

func Connect() *Connection {
	connInfo := fmt.Sprintf("host=%s port=%s user=%s "+
		"password=%s dbname=%s sslmode=disable",
		os.Getenv("POSTGRESS_ADDRESS"), os.Getenv("POSTGRES_PORT"), os.Getenv("POSTGRES_USER"), os.Getenv("POSTGRES_PASSWORD"), os.Getenv("POSTGRES_DB"))
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
	query := `INSERT INTO expressions(expressionid, expression, status, userid) VALUES (@expressionId, @expression, @status, @userid) returning expressionid`
	args := pgx.NamedArgs{
		"expressionId": id,
		"expression":   expr,
		"status":       0,
		"userid":       ctx.Value("userid"),
	}
	_, err := c.conn.Exec(ctx, query, args)
	if err != nil {
		return fmt.Errorf("unable to insert row: %w", err)
	}
	return nil
}

func (c *Connection) GetExpressions(ctx context.Context) ([]struct {
	Uuid   string      `json:"expressionid"`
	Expr   string      `json:"expression"`
	Status int         `json:"status"`
	Result interface{} `json:"result"`
}, error) {
	query := `SELECT expressionid, expression, status, result FROM expressions where userid = $1`
	rows, err := c.conn.Query(ctx, query, ctx.Value("userid"))
	if err != nil {
		return []struct {
			Uuid   string      `json:"expressionid"`
			Expr   string      `json:"expression"`
			Status int         `json:"status"`
			Result interface{} `json:"result"`
		}{}, fmt.Errorf("unable to query expressions: %w", err)
	}
	defer rows.Close()
	exprs := []struct {
		Uuid   string      `json:"expressionid"`
		Expr   string      `json:"expression"`
		Status int         `json:"status"`
		Result interface{} `json:"result"`
	}{}
	for rows.Next() {
		expr := struct {
			Uuid   string      `json:"expressionid"`
			Expr   string      `json:"expression"`
			Status int         `json:"status"`
			Result interface{} `json:"result"`
		}{}
		err := rows.Scan(&expr.Uuid, &expr.Expr, &expr.Status, &expr.Result)
		if err != nil {
			return []struct {
				Uuid   string      `json:"expressionid"`
				Expr   string      `json:"expression"`
				Status int         `json:"status"`
				Result interface{} `json:"result"`
			}{}, fmt.Errorf("unable to scan row: %w", err)
		}
		exprs = append(exprs, expr)
	}

	return exprs, nil
}

func (c *Connection) GetExpressionByID(ctx context.Context, expressionid string) (interface{}, int32, error) {
	// ctxWithT, cancel := context.WithTimeout(ctx, time.Second*2)
	// defer cancel()
	query := `SELECT result, status FROM expressions where expressionid = @expressionId and userid = @userid`
	args := pgx.NamedArgs{
		"expressionId": expressionid,
		"userid":       ctx.Value("userid"),
	}
	rows, err := c.conn.Query(ctx, query, args)
	if err != nil {
		return "", 0, fmt.Errorf("unable to query expression: %w", err)
	}
	defer rows.Close()
	var result interface{}
	var status interface{}
	for rows.Next() {
		err := rows.Scan(&result, &status)
		if err != nil {
			return "", 0, fmt.Errorf("unable to scan row: %w", err)
		}
	}
	if status == nil {
		return "", 0, fmt.Errorf("expression didn't exist")
	}
	st, _ := status.(int32)
	return result, st, nil
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
				slog.Info(fmt.Sprintf("operation %s already exists", task.OperationID))
				continue
			}
			slog.Info(fmt.Sprint(task.ExpressionID, task.OperationID, task.ParentID))
			return fmt.Errorf("unable to insert row: %w", err)
		}
	}
	return results.Close()
}

func (c *Connection) ChangeOperationStatus(ctx context.Context, operationid string, status int) error {
	query := `UPDATE operations SET status = @status, changedtime = @time WHERE operationid = @operationid`
	args := pgx.NamedArgs{
		"operationid": operationid,
		"status":      status,
		"time":        time.Now(),
	}
	_, err := c.conn.Exec(ctx, query, args)
	if err != nil {
		return fmt.Errorf("unable to update row: %w", err)
	}
	slog.Info(fmt.Sprintf("Changed operation %s status to %d", operationid, status))
	return nil
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
	slog.Info(fmt.Sprintf("Changed expression %s status to %d", expressionid, status))
	return nil
}

func (c *Connection) GetOperationsToExecution(ctx context.Context) ([]calc.Operation, error) {
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
	now := time.Now()
	query := `UPDATE operations SET status = @status, changedtime = @time where operationid = @operationid`
	batch := &pgx.Batch{}
	for _, task := range operations {
		args := pgx.NamedArgs{
			"operationid": task.OperationID,
			"status":      status,
			"time":        now,
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
	slog.Info(fmt.Sprintf("Get operation (%s) result: %f", operationid, result))
	return nil
}

func (c *Connection) SetExpressionResult(ctx context.Context, expressionid string, result float64) error {
	query := `UPDATE expressions SET result = @result where expressionid = @expressionid`
	args := pgx.NamedArgs{
		"expressionid": expressionid,
		"result":       result,
	}
	_, err := c.conn.Exec(ctx, query, args)
	if err != nil {
		return fmt.Errorf("unable to update row: %w", err)
	}
	slog.Info(fmt.Sprintf("Get expression (%s) result: %f", expressionid, result))
	return nil
}

func (c *Connection) GetComplitedOperation(ctx context.Context) ([]calc.Operation, error) {
	query := `SELECT operationid, expressionid, parentid, "left", result FROM operations where status = 1 and result is not null`
	rows, err := c.conn.Query(ctx, query)
	if err != nil {
		return []calc.Operation{}, fmt.Errorf("unable to query operations: %w", err)
	}
	defer rows.Close()
	result := []calc.Operation{}
	for rows.Next() {
		var res = calc.Operation{}
		err := rows.Scan(&res.OperationID, &res.ExpressionID, &res.ParentID, &res.Left, &res.Result)
		if err != nil {
			return []calc.Operation{}, fmt.Errorf("unable to scan row: %w", err)
		}
		result = append(result, res)
	}
	return result, nil
}

func (c *Connection) SetOperationResultToParent(ctx context.Context, opers []struct {
	Operationid string
	Parentid    string
	Res         float64
	Left        bool
}) error {
	batch := &pgx.Batch{}
	var query string
	for _, op := range opers {
		if op.Left {
			query = `UPDATE operations SET v1 = @result where operationid = @parentid`
		} else {
			query = `UPDATE operations SET v2 = @result where operationid = @parentid`
		}
		args := pgx.NamedArgs{
			"operationid": op.Operationid,
			"result":      op.Res,
			"parentid":    op.Parentid,
		}
		batch.Queue(query, args)
	}
	results := c.conn.SendBatch(ctx, batch)
	defer results.Close()
	for _, op := range opers {
		_, err := results.Exec()
		if err != nil {
			slog.Info(fmt.Sprint(op.Parentid, op.Left))
			return fmt.Errorf("unable to update row: %w", err)
		}
		slog.Info(fmt.Sprintf("Update %s", op.Operationid))
	}
	return results.Close()
}

func (c *Connection) Registration(ctx context.Context, username string, hash string) error {
	query := `INSERT INTO users (username, hash) VALUES (@username, @hash) returning username`
	args := pgx.NamedArgs{
		"username": username,
		"hash":     hash,
	}
	rows, err := c.conn.Query(ctx, query, args)
	if err != nil {
		return fmt.Errorf("unable to insert user row: %w", err)
	}
	defer rows.Close()
	var resp string
	for rows.Next() {
		rows.Scan(&resp)
	}
	if resp != username {
		return fmt.Errorf("unexpected error")
	}
	return nil
}

func (c *Connection) Login(ctx context.Context, username, password string) (bool, error) {
	query := `Select username, hash from users where username = @username`
	args := pgx.NamedArgs{
		"username": username,
	}
	rows, err := c.conn.Query(ctx, query, args)
	if err != nil {
		return false, fmt.Errorf("unable to login: %w", err)
	}
	defer rows.Close()
	resp := struct {
		username string
		hash     string
	}{}
	for rows.Next() {
		rows.Scan(&resp.username, &resp.hash)
	}
	if resp.username != username {
		return false, nil
	}
	err = bcrypt.CompareHashAndPassword([]byte(resp.hash), []byte(password))
	return err == nil, err
}

func (c *Connection) GetUserID(ctx context.Context, username string) (int, error) {
	query := "Select id from users where username = @username"
	args := pgx.NamedArgs{
		"username": username,
	}
	rows, err := c.conn.Query(ctx, query, args)
	if err != nil {
		return -1, fmt.Errorf("unable to login: %w", err)
	}
	defer rows.Close()
	id := -1
	for rows.Next() {
		rows.Scan(&id)
	}
	if id == -1 {
		return -1, nil
	}
	return id, nil
}

func (c *Connection) UpdateStuckedOperations(ctx context.Context, timeout time.Duration) error {
	query := `UPDATE operations set status=0 where (status = 1 and result is null and @time - changedtime > @delta) or (status = 2 and result is null)`
	args := pgx.NamedArgs{
		"time":  time.Now(),
		"delta": timeout,
	}
	_, err := c.conn.Exec(ctx, query, args)
	if err != nil {
		return fmt.Errorf("unable to insert row: %w", err)
	}
	return nil
}

func (c *Connection) GetUserIDFromOperationId(ctx context.Context, operationId string) (int, error) {
	query := `SELECT e.userid
	FROM public.operations o
	JOIN public.expressions e ON o.expressionid = e.expressionid
	WHERE o.operationid = @operationid;`
	args := pgx.NamedArgs{
		"operationid": operationId,
	}
	rows, err := c.conn.Query(ctx, query, args)
	if err != nil {
		return -1, fmt.Errorf("unable to login: %w", err)
	}
	defer rows.Close()
	id := -1
	for rows.Next() {
		rows.Scan(&id)
	}
	if id == -1 {
		return -1, nil
	}
	return id, nil
}
func (c *Connection) GetAllUserId(ctx context.Context) ([]int, error) {
	query := `select id from users`
	rows, err := c.conn.Query(ctx, query)
	if err != nil {
		return []int{}, fmt.Errorf("unable to login: %w", err)
	}
	defer rows.Close()
	res := []int{}
	id := -1
	for rows.Next() {
		rows.Scan(&id)
		res = append(res, id)
	}
	if id == -1 {
		return []int{}, nil
	}
	return res, nil
}
