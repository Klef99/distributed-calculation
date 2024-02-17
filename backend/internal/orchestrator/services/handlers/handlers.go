package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/klef99/distributed-calculation-backend/internal/orchestrator/database"
	"github.com/klef99/distributed-calculation-backend/pkg/calc"
)

type Expression struct {
	Expressionid string `json:"expressionid"`
	Expr         string `json:"expression"`
	Status       int    `json:"status"`
}

type Handler struct {
	conn *database.Connection
}

func New(db *database.Connection) Handler {
	return Handler{conn: db}
}

func (h *Handler) AddExpression(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	var expr string
	expr = r.URL.Query().Get("expression")
	expr, err := calc.ValidExpression(expr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		slog.Info(err.Error())
		return
	}
	res := Expression{Expressionid: uuid.NewString(), Expr: expr, Status: 0}
	err = h.conn.InsertExpression(r.Context(), res.Expressionid, res.Expr)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		// json.NewEncoder(w).Encode(res)
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
	result, err := h.conn.GetExpressionByID(r.Context(), exprId)
	res := struct {
		ExpressionId string      `json:"expressionId"`
		Res          interface{} `json:"result"`
	}{Res: result, ExpressionId: exprId}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		slog.Warn(err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(res)
}
