package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/klef99/distributed-calculation-backend/internal/orchestrator/database"
	"github.com/klef99/distributed-calculation-backend/pkg/calc"
	"github.com/klef99/distributed-calculation-backend/pkg/redis"
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
	_, _, err = h.conn.GetExpressionByID(r.Context(), expressionid)
	if err == nil {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Expression exist in database"))
		return
	}
	res := Expression{Expressionid: expressionid, Expr: expr, Status: 0}
	err = h.conn.InsertExpression(r.Context(), res.Expressionid, res.Expr)
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
	exprs, err := h.conn.GetExpressions(r.Context())
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
	result, status, err := h.conn.GetExpressionByID(r.Context(), exprId)
	res := struct {
		ExpressionId string      `json:"expressionId"`
		Status       int32       `json:"status"`
		Res          interface{} `json:"result"`
	}{Res: result, ExpressionId: exprId, Status: status}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
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
	err = h.connR.BulkSetOperationsTimeouts(operationTimeout)
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
	timeouts, err := h.connR.GetOperationsTimeouts()
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
