package orchestrator

import (
	"context"
	"sync"

	"beryl7-agent/logger"
)

type DBTask func() error

// DBWriterQueue guarantees single-threaded sequential execution of SQLite writes, tripe-preventing SQLITE_BUSY lock contention
type DBWriterQueue struct {
	taskChan chan DBTask
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func NewDBWriterQueue(bufferSize int) *DBWriterQueue {
	if bufferSize <= 0 {
		bufferSize = 128
	}
	ctx, cancel := context.WithCancel(context.Background())
	q := &DBWriterQueue{
		taskChan: make(chan DBTask, bufferSize),
		ctx:      ctx,
		cancel:   cancel,
	}

	q.wg.Add(1)
	go q.workerLoop()

	return q
}

func (q *DBWriterQueue) workerLoop() {
	defer q.wg.Done()
	logger.Info("DBWriterQueue: Single-threaded SQLite Write Worker started.")

	for {
		select {
		case <-q.ctx.Done():
			// Process remaining tasks in queue before exiting
			for len(q.taskChan) > 0 {
				task := <-q.taskChan
				q.executeTask(task)
			}
			logger.Info("DBWriterQueue: Worker stopped cleanly.")
			return
		case task, ok := <-q.taskChan:
			if !ok {
				return
			}
			q.executeTask(task)
		}
	}
}

func (q *DBWriterQueue) executeTask(task DBTask) {
	if task == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			logger.Error("DBWriterQueue: DB task panic recovered: %v", r)
		}
	}()

	if err := task(); err != nil {
		logger.Warn("DBWriterQueue: Async DB task returned error: %v", err)
	}
}

// Enqueue submits a database task to the sequential worker queue
func (q *DBWriterQueue) Enqueue(task DBTask) bool {
	select {
	case <-q.ctx.Done():
		return false
	case q.taskChan <- task:
		return true
	default:
		logger.Warn("DBWriterQueue: Task buffer full (128), task dropped to prevent blocking main thread")
		return false
	}
}

// Stop gracefully shuts down the queue worker after draining pending tasks
func (q *DBWriterQueue) Stop() {
	q.cancel()
	q.wg.Wait()
}
