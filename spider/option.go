package spider

// OptionFunc Spider 配置func
type OptionFunc func(*Spider)

// SetHandlerBeforeAddTask 设置预处理before 增加任务
func SetHandlerBeforeAddTask(f TaskHandler) OptionFunc {
	return func(s *Spider) { s.beforeAddTask = f }
}

// SetHandlerAfterTaskDone 设置任务处理器当Task执行完成后
func SetHandlerAfterTaskDone(f TaskHandler) OptionFunc {
	return func(s *Spider) { s.afterTaskDone = f }
}
