package orchestrator

import (
	"sync"

	"beryl7-agent/logger"
)

type DBTask func() error

// DBWriterQueue guarantees single-threaded sequential execution of SQLite writes, tripe-preventing SQLITE_BUSY lock contention
type DBWriterQueue struct {
	taskChan chan DBTask
	stopped  bool
	mu       sync.RWMutex
	wg       sync.WaitGroup
}

func NewDBWriterQueue(bufferSize int) *DBWriterQueue {
	if bufferSize <= 0 {
		bufferSize = 128
	}
	q := &DBWriterQueue{
		taskChan: make(chan DBTask, bufferSize),
	}

	q.wg.Add(1)
	go q.workerLoop()

	return q
}

// [Fix 2] Worker loop ranges over taskChan directly, guaranteeing 100% deterministic task draining on queue stop without task loss
func (q *DBWriterQueue) workerLoop() {
	defer q.wg.Done()
	logger.Info("DBWriterQueue: Single-threaded SQLite Write Worker started.")

	for task := range q.taskChan {
		q.executeTask(task)
	}
	logger.Info("DBWriterQueue: Worker stopped cleanly after draining queue.")
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

// [Fix 2] Mutex-protected stopped check prevents task-loss and pseudo-random select race conditions during shutdown
func (q *DBWriterQueue) Enqueue(task DBTask) bool {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if q.stopped {
		return false
	}

	select {
	case q.taskChan <- task:
		return true
	default:
		logger.Warn("DBWriterQueue: Task buffer full (128), task dropped to prevent blocking main thread")
		return false
	}
}

// [Fix 2] Graceful Stop closes taskChan under mutex lock and waits for worker to finish draining all pending tasks
func (q *DBWriterQueue) Stop() {
	q.mu.Lock()
	if q.stopped {
		q.mu.Unlock()
		return
	}
	q.stopped = true
	close(q.taskChan)
	q.mu.Unlock()

	q.wg.Wait()
}
