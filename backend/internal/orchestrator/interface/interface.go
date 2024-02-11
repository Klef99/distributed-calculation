package _interface

import (
	"encoding/json"
	"net/http"
)

type Expression struct {
	ID       string `json:"id"`
	MathExpr string `json:"math_expr"`
	Status   string `json:"status"`
	Result   int    `json:"result"`
}

func addExpression(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var newExpression Expression
	err := json.NewDecoder(r.Body).Decode(&newExpression)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Process and queue the new expression for computation

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newExpression)
}
