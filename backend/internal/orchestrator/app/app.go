package app

import (
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/klef99/distributed-calculation-backend/internal/orchestrator/services/distributor"
	"github.com/klef99/distributed-calculation-backend/internal/orchestrator/services/handlers"
	"github.com/klef99/distributed-calculation-backend/pkg/database"
	"github.com/klef99/distributed-calculation-backend/pkg/redis"
)

func Run() {
	// Установим соединения с storage
	conn := database.Connect()
	RedisConn := redis.NewConnectionRedis()
	defer redis.CloseConnectionRedis(RedisConn)
	defer database.CloseConnection(conn)

	// timeouts, err := RedisConn.GetOperationsTimeouts()
	// defTimeouts := map[string]int{"+": 10, "-": 10, "*": 10, "/": 10}
	// if err != nil {
	// 	err = RedisConn.BulkSetOperationsTimeouts(defTimeouts)
	// 	if err != nil {
	// 		panic("unable to set timeouts")
	// 	}
	// } else {
	// 	for _, op := range []string{"+", "-", "/", "*"} {
	// 		flag := false
	// 		var k string
	// 		var dur time.Duration
	// 		for k, dur = range timeouts {
	// 			if k == op {
	// 				flag = true
	// 				break
	// 			}
	// 		}
	// 		if flag {
	// 			defTimeouts[k] = int(dur.Seconds())
	// 		}
	// 	}
	// 	err = RedisConn.BulkSetOperationsTimeouts(defTimeouts)
	// 	if err != nil {
	// 		panic("unable to set timeouts")
	// 	}
	// }
	// Создадим структуры-провайдоры запросов к бд и агентам
	h := handlers.New(conn, RedisConn)
	d := distributor.NewDistributor(RedisConn, conn)
	// Запустим операции
	go d.NewOperations(2 * time.Second)
	go d.SendOperations(2 * time.Second)
	go d.GetOperationResult()
	go d.UpdateOperations(2 * time.Second)
	go d.RestoreStuckedOperation(1 * time.Minute)
	// Создаём http-сервер
	router := mux.NewRouter()
	router.HandleFunc("/addExpression", h.AuthMW(h.AddExpression))
	router.HandleFunc("/getExpressionsList", h.AuthMW(h.GetExpressionsList))
	router.HandleFunc("/getExpressionByID", h.AuthMW(h.GetExpressionByID))
	router.HandleFunc("/register", h.Registration)
	router.HandleFunc("/login", h.Login)
	router.HandleFunc("/setOperationsTimeout", h.AuthMW(h.SetOperationsTimeout))
	router.HandleFunc("/getOperationsTimeout", h.AuthMW(h.GetOperationsTimeout))
	// Внутренние операции, не предназначенные для пользователя
	router.HandleFunc("/getHearthbeat", h.GetHearthbeat)
	router.HandleFunc("/getWorkersStatus", h.GetWorkersStatus)
	err := http.ListenAndServe(":8080", router)
	if err != nil {
		log.Fatalln("There's an error with the server", err)
	}
}
