package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/klef99/distributed-calculation-backend/pkg/calc"
)

type Expression struct {
	ID     string `json:"id"`
	Expr   string `json:"math_expr"`
	Status string `json:"status"`
	Result int    `json:"result"`
}

func AddExpression(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	var expr string
	expr = r.URL.Query().Get("expression")
	expr, err := calc.ValidExpression(expr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	res := Expression{ID: uuid.NewString(), Expr: expr, Status: "registered"}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(res)
	// fmt.Fprint(w, "Hello, world!")
}
