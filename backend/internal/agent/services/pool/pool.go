package pool

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/klef99/distributed-calculation-backend/pkg/calc"
)

// Worker Интерфейс надо реализовать объектам, которые будут обрабатываться параллельно
type Worker interface {
	Task(operTimeouts map[string]time.Duration) float64
}

// Pool Пул для выполнения
type Pool struct {
	// из этого канала будем брать задачи для обработки
	tasks   chan Worker
	Results chan struct {
		OperationID string
		Res         float64
	}
	timeouts map[string]time.Duration
	// для синхронизации работы
	wg         sync.WaitGroup
	mu         sync.Mutex
	countTasks atomic.Int32
}

// New при создании пула передадим максимальное количество горутин
func New(maxGoroutines int) *Pool {
	p := Pool{
		tasks: make(chan Worker), // канал, откуда брать задачи
		Results: make(chan struct {
			OperationID string
			Res         float64
		}),
		countTasks: atomic.Int32{},
	}
	// для ожидания завершения
	p.wg.Add(maxGoroutines)
	for i := 0; i < maxGoroutines; i++ {
		// создадим горутины по указанному количеству maxGoroutines
		go func() {
			// забираем задачи из канала
			for w := range p.tasks {
				// и выполняем
				p.countTasks.Add(1)
				operationID := w.(calc.Operation).OperationID
				res := w.Task(p.timeouts)
				p.Results <- struct {
					OperationID string
					Res         float64
				}{OperationID: operationID, Res: res}
				p.countTasks.Add(-1)
			}
			// после закрытия канала нужно оповестить наш пул
			p.wg.Done()
		}()
	}

	return &p
}

// Передаем объект, который реализует интерфейс Worker и добавляем задачи в канал, из которого забирает работу пул
func (p *Pool) Run(w Worker, timeouts map[string]time.Duration) {
	p.tasks <- w
	p.mu.Lock()
	p.timeouts = timeouts
	p.mu.Unlock()
}

func (p *Pool) Shutdown() {
	// закроем канал с задачами
	close(p.tasks)
	// дождемся завершения работы уже запущенных задач
	p.wg.Wait()
	close(p.Results)
}

func (p *Pool) SendHearthbeat(workerName string, tick time.Duration) {
	ticker := time.NewTicker(tick)
	for range ticker.C {
		hearthbeat := struct {
			WorkerName       string `json:"workerName"`
			TaskCountCurrent int    `json:"taskCountCurrent"`
		}{WorkerName: workerName, TaskCountCurrent: int(p.countTasks.Load())}
		data, _ := json.Marshal(hearthbeat)
		r := bytes.NewReader(data)
		resp, err := http.Post(fmt.Sprintf("http://%s:%s/getHearthbeat", os.Getenv("ORCHESTRATOR_ADDRESS"), os.Getenv("ORCHESTRATOR_PORT")), "application/json", r)
		if err != nil || resp.StatusCode != http.StatusOK {
			slog.Warn("orchestrator didn't work properly")
		}
	}
}
