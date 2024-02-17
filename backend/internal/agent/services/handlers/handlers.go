package handlers

import "net/http"

func HearthbeatHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func GetTasksHandler(w http.ResponseWriter, r *http.Request) {

}
