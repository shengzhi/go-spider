package spider

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	userAgent      = "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/56.0.2924.87 Safari/537.36"
	acceptEncoding = "gzip, deflate"
	accept         = "*/*"
	acceptLang     = "zh-CN,zh;q=0.8,en;q=0.6,zh-TW;q=0.4"
)

// TaskContext 爬取任务上下文
type TaskContext struct {
	spider   *Spider
	task     *Task
	Err      error
	Response *http.Response
}

// Current 返回当前任务
func (tc *TaskContext) Current() *Task { return tc.task }

// ExtData 返回任务所携带附加数据
func (tc *TaskContext) ExtData() interface{} {
	return tc.task.ExtData
}

// AddTask 增加任务
func (tc *TaskContext) AddTask(tasks ...*Task) {
	// fmt.Println("add new tasks number:", len(tasks))
	for _, t := range tasks {
		tc.spider.taskManager.Enqueue(t)
	}
}

// Output 输出解析后的数据
func (tc *TaskContext) Output(values ...interface{}) {
	tc.spider.genData(values...)
}

// Data 返回Http Response Body
func (tc *TaskContext) Data() ([]byte, error) {
	if tc.Response == nil {
		return []byte{}, nil
	}
	defer tc.Response.Body.Close()
	var reader io.Reader = tc.Response.Body
	contentEncoding := strings.ToLower(tc.Response.Header.Get("Content-Encoding"))
	if strings.Contains(contentEncoding, "gzip") {
		zr, err := gzip.NewReader(tc.Response.Body)
		if err != nil {
			return nil, err
		}
		reader = zr
	}
	if strings.Contains(contentEncoding, "deflate") {
		rc := flate.NewReader(tc.Response.Body)
		defer rc.Close()
		reader = rc
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, reader); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Handler 爬取结果处理器
type Handler func(c *TaskContext)

// Task 爬取任务
type Task struct {
	URL, Method string
	Data        io.Reader
	Header      http.Header
	Cookies     []*http.Cookie
	Handler     Handler
	ExtData     interface{}
	Retry       byte
}

func (t *Task) String() string {
	return fmt.Sprintf("%s %s %s %v", time.Now().Format("2006-01-02 15:04:05"), t.URL, t.Method, t.ExtData)
}

// SetDefaultHeader 设置默认http header
func (t *Task) SetDefaultHeader() {
	t.Header.Set("User-Agent", userAgent)
	t.Header.Set("Accept-Encoding", acceptEncoding)
	t.Header.Set("Accept", accept)
	t.Header.Set("Accept-Language", acceptLang)
}

func (t *Task) CopyHeader(header http.Header) {
	for k, v := range header {
		t.Header.Set(k, strings.Join(v, ";"))
	}
}

func (t *Task) CopyCookie(cookies []*http.Cookie) {
	t.Cookies = append(t.Cookies, cookies...)
}

// NewTask 创建爬取任务，GET 操作
func NewTask(url string, handler Handler) *Task {
	t := &Task{URL: url, Method: "GET", Retry: 3, Handler: handler, Header: http.Header{}}
	t.SetDefaultHeader()
	return t
}

// DataProcesser 数据处理接口
type DataProcesser interface {
	Process(ch <-chan interface{})
}

// DataHandlerFunc 包装方法为数据处理接口
func DataHandlerFunc(fn func(ch <-chan interface{})) DataHandler {
	return DataHandler(fn)
}

// DataHandler 数据处理方法
type DataHandler func(ch <-chan interface{})

// Process 实现接口DataProcesser
func (dh DataHandler) Process(ch <-chan interface{}) {
	dh(ch)
}

// Spider 蜘蛛
type Spider struct {
	taskManager   TaskManager
	timeout       time.Duration
	fetcher       *Fetcher
	chData        chan interface{}
	processer     DataProcesser
	taskNum       int
	iscompleted   chan struct{}
	done, isdebug bool
}

// NewSpider 创建爬虫实例
func NewSpider(dp DataProcesser) *Spider {
	s := &Spider{
		chData:      make(chan interface{}, 1000),
		processer:   dp,
		iscompleted: make(chan struct{}),
	}
	return s
}

// Init 初始化入口地址
func (s *Spider) Init(entry string, handler Handler) {
	s.taskManager = newDefaultTaskManager(make(chan *Task, 100))
	s.taskManager.Enqueue(NewTask(entry, handler))
}

// InitFunc 初始化入口方法
func (s *Spider) InitFunc(fn func() chan *Task) {
	s.taskManager = newDefaultTaskManager(fn())
}

// Execute 立即执行任务
func (s *Spider) Execute(t *Task) (*http.Response, error) {
	res, err := newFectcher(nil, nil, time.Second*30).httpCall(t)
	return res, err
}

// SetTimeout 设置超时时长
func (s *Spider) SetTimeout(d time.Duration) { s.timeout = d }

// SetTaskNum 设置工作线程数量，默认为CPU核数
func (s *Spider) SetTaskNum(n int) { s.taskNum = n }

// Run 开爬
func (s *Spider) Run() {
	out := make(chan *TaskContext, 50)
	s.fetcher = newFectcher(s.taskManager.Chan(), out, s.timeout)
	s.fetcher.isdebug = s.isdebug

	go func() {
		for tc := range out {
			if tc.Err != nil && tc.task.Retry > 0 {
				tc.task.Retry--
				s.taskManager.Enqueue(tc.task)
				continue
			}
			tc.spider = s
			tc.task.Handler(tc)
		}
	}()
	wg := &sync.WaitGroup{}
	if s.taskNum == 0 {
		s.taskNum = runtime.NumCPU()
	}
	for i := 0; i < s.taskNum; i++ {
		wg.Add(1)
		go s.fetcher.Run(wg)
	}
	if s.processer != nil {
		go func() {
			s.processer.Process(s.chData)
			s.iscompleted <- struct{}{}
		}()
	}
	wg.Wait()
	s.done = true
	close(s.chData)
	<-s.iscompleted
}

// genData 输出数据
func (s *Spider) genData(values ...interface{}) {
	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
		}
	}()
	if s.done {
		return
	}
	for _, v := range values {
		s.chData <- v
	}
}

func (s *Spider) EnableDebug() { s.isdebug = true }
