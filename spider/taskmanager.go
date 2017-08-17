// Task manager

package spider

// TaskManager URL管理器
type TaskManager interface {
	// Chan 队列
	Chan() <-chan *Task
	// Enqueue 入队
	Enqueue(task *Task)
}

type defaultTaskManager struct {
	q chan *Task
}

func newDefaultTaskManager(ch chan *Task) *defaultTaskManager {
	return &defaultTaskManager{q: ch}
}

// Dequeue 获取任务
func (m *defaultTaskManager) Chan() <-chan *Task {
	return m.q
}

// Enqueue 添加任务
func (m *defaultTaskManager) Enqueue(task *Task) {
	m.q <- task
}
