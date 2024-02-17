package pool

import (
	"sync"

	"github.com/klef99/distributed-calculation-backend/pkg/calc"
)

// Worker Интерфейс надо реализовать объектам, которые будут обрабатываться параллельно
type Worker interface {
	Task() float64
}

// Pool Пул для выполнения
type Pool struct {
	// из этого канала будем брать задачи для обработки
	tasks   chan Worker
	Results chan struct {
		OperationID string
		Res         float64
	}
	// для синхронизации работы
	wg sync.WaitGroup
}

// New при создании пула передадим максимальное количество горутин
func New(maxGoroutines int) *Pool {
	p := Pool{
		tasks: make(chan Worker), // канал, откуда брать задачи
		Results: make(chan struct {
			OperationID string
			Res         float64
		}),
	}
	// для ожидания завершения
	p.wg.Add(maxGoroutines)
	for i := 0; i < maxGoroutines; i++ {
		// создадим горутины по указанному количеству maxGoroutines
		go func() {
			// забираем задачи из канала
			for w := range p.tasks {
				// и выполняем
				operationID := w.(calc.Operation).OperationID
				res := w.Task()
				p.Results <- struct {
					OperationID string
					Res         float64
				}{OperationID: operationID, Res: res}
			}
			// после закрытия канала нужно оповестить наш пул
			p.wg.Done()
		}()
	}

	return &p
}

// Run Передаем объект, который реализует интерфейс Worker и добавляем задачи в канал, из которого забирает работу пул
func (p *Pool) Run(w Worker) {
	p.tasks <- w
}

func (p *Pool) Shutdown() {
	// закроем канал с задачами
	close(p.tasks)
	// дождемся завершения работы уже запущенных задач
	p.wg.Wait()
	close(p.Results)
}
