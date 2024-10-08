package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/klef99/distributed-calculation-backend/internal/orchestrator/services/jwtgenerator"
	"github.com/klef99/distributed-calculation-backend/pkg/calc"
	"github.com/klef99/distributed-calculation-backend/pkg/database"
	"github.com/klef99/distributed-calculation-backend/pkg/redis"
	"golang.org/x/crypto/bcrypt"
)

type Expression struct {
	Expressionid string `json:"expressionid"`
	Expr         string `json:"expression"`
	Status       int    `json:"status"`
}

type Handler struct {
	conn  *database.Connection
	connR *redis.ConnectionRedis
}

func New(db *database.Connection, red *redis.ConnectionRedis) Handler {
	return Handler{conn: db, connR: red}
}

func (h *Handler) AddExpression(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	exprs := struct {
		Expression string `json:"expression"`
	}{}
	dec := json.NewDecoder(r.Body)
	err := dec.Decode(&exprs)
	userid, _ := strconv.Atoi(r.Header.Get("userid"))
	nctx := context.WithValue(r.Context(), "userid", userid)
	if exprs.Expression == "" || err != nil {
		w.WriteHeader(http.StatusBadRequest)
		slog.Info("wrong decode expression")
		return
	}
	expr, err := calc.ValidExpression(exprs.Expression)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		slog.Info(err.Error())
		return
	}
	expressionid := r.Header.Get("X-Request-Id")
	if expressionid == "" {
		expressionid = uuid.NewString()
	}
	_, _, _, err = h.conn.GetExpressionByID(nctx, expressionid)
	if err == nil {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Expression exist in database"))
		return
	}
	res := Expression{Expressionid: expressionid, Expr: expr, Status: 0}
	err = h.conn.InsertExpression(nctx, res.Expressionid, res.Expr)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		slog.Warn(err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(res)
}

func (h *Handler) GetExpressionsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	userid, _ := strconv.Atoi(r.Header.Get("userid"))
	nctx := context.WithValue(r.Context(), "userid", userid)
	exprs, err := h.conn.GetExpressions(nctx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		slog.Warn(err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(exprs)
}

func (h *Handler) GetExpressionByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	exprId := r.URL.Query().Get("expressionId")
	userid, _ := strconv.Atoi(r.Header.Get("userid"))
	nctx := context.WithValue(r.Context(), "userid", userid)
	expression, result, status, err := h.conn.GetExpressionByID(nctx, exprId)
	res := struct {
		Uuid   string      `json:"expressionid"`
		Expr   string      `json:"expression"`
		Status int32       `json:"status"`
		Result interface{} `json:"result"`
	}{Result: result, Uuid: exprId, Status: status, Expr: expression}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if err.Error() == "expression didn't exist" {
			w.Write([]byte(err.Error()))
		}
		slog.Warn(err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(res)
}

func (h *Handler) GetHearthbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	hearthbeat := struct {
		WorkerName       string `json:"workerName"`
		TaskCountCurrent int    `json:"taskCountCurrent"`
	}{}
	data, _ := io.ReadAll(r.Body)
	json.Unmarshal(data, &hearthbeat)
	h.connR.SetWorkerStatus(r.Context(), hearthbeat.WorkerName, hearthbeat.TaskCountCurrent)
	slog.Info(fmt.Sprintf("Get herathbeat from %s. Num of task: %d", hearthbeat.WorkerName, hearthbeat.TaskCountCurrent))
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) GetWorkersStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	data, err := h.connR.GetWorkersStatus(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		slog.Warn(err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) SetOperationsTimeout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	var operationTimeout map[string]int
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&operationTimeout)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		slog.Warn(err.Error())
		return
	}
	userid, _ := strconv.Atoi(r.Header.Get("userid"))
	err = h.connR.BulkSetOperationsTimeouts(operationTimeout, userid)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		slog.Warn(err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (h *Handler) GetOperationsTimeout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	userid, _ := strconv.Atoi(r.Header.Get("userid"))
	timeouts, err := h.connR.GetOperationsTimeouts(userid)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		slog.Warn(err.Error())
		return
	}
	ret := make(map[string]int, 0)
	for k, v := range timeouts {
		ret[k] = int(v.Seconds())
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ret)
}

func (h *Handler) Registration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	user := struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&user)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Wrong format of request body"))
		slog.Warn(err.Error())
		return
	}
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		slog.Warn(err.Error())
		return
	}
	err = h.conn.Registration(context.Background(), user.Login, string(hashedBytes))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("User already exist or registration failed"))
		slog.Warn(err.Error())
		return
	}
	userid, err := h.conn.GetUserID(context.Background(), user.Login)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		slog.Warn(err.Error())
		return
	}
	err = h.connR.BulkSetOperationsTimeouts(map[string]int{"+": 10, "-": 10, "*": 10, "/": 10}, userid)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		slog.Warn(err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	user := struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&user)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Wrong format of request body"))
		slog.Warn(err.Error())
		return
	}
	isLogin, err := h.conn.Login(context.Background(), user.Login, user.Password)
	if !isLogin {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Invalid username or password"))
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		slog.Warn(err.Error())
		w.Write([]byte("Failed to log in."))
		return
	}
	newToken, err := jwtgenerator.GenerateToken(user.Login)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		slog.Warn(err.Error())
		w.Write([]byte("Unable to create."))
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(newToken))
}

func (h *Handler) AuthMW(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// read basic auth information
		bearerToken := r.Header.Get("Authorization")
		reqToken := strings.Split(bearerToken, " ")
		if len(reqToken) != 2 {
			w.WriteHeader(http.StatusUnauthorized)
			slog.Warn("Invalid request (required authorization token)")
			w.Write([]byte("Invalid request (required authorization token)"))
			return
		}
		username, err := jwtgenerator.ValidateToken(reqToken[1])
		if username == "" || err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			slog.Warn(err.Error())
			w.Write([]byte(err.Error()))
			return
		}
		userid, err := h.conn.GetUserID(context.Background(), username)
		if userid == -1 || err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			slog.Warn(err.Error())
			return
		}
		r.Header.Add("userid", strconv.Itoa(userid))
		next(w, r)
	}
}
