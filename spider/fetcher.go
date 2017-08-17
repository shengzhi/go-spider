// HTTP抓取模块

package spider

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"
	"time"
)

// Fetcher http执行器
type Fetcher struct {
	in          <-chan *Task
	out         chan<- *TaskContext
	timeount    time.Duration
	httpclient  *http.Client
	httpsclient *http.Client
	isdebug     bool
}

func newFectcher(in <-chan *Task, out chan<- *TaskContext, timeout time.Duration) *Fetcher {
	f := &Fetcher{in: in, out: out, timeount: timeout}
	f.httpclient = &http.Client{Timeout: f.timeount}
	f.httpclient.Transport = &http.Transport{
		DisableKeepAlives: false}

	f.httpsclient = &http.Client{Timeout: f.timeount}
	f.httpsclient.Transport = &http.Transport{
		DisableKeepAlives: false,
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
	}
	return f
}

// Run 执行http请求
func (f *Fetcher) Run(wg *sync.WaitGroup) {
LOOP:
	for {
		select {
		case task := <-f.in:
			// fmt.Println("http call begin ", task.URL)
			res, err := f.httpCall(task)
			if err != nil {
				log.Println(err)
			}
			// fmt.Println("http call end ", task.URL)
			f.out <- &TaskContext{
				task:     task,
				Response: res,
				Err:      err,
			}
		case <-time.After(time.Minute * 10):
			fmt.Println("done...")
			wg.Done()
			break LOOP
		}
	}
}

func (f *Fetcher) httpCall(t *Task) (*http.Response, error) {
	r, err := http.NewRequest(t.Method, t.URL, t.Data)
	if err != nil {
		return nil, fmt.Errorf("Initialize request occurs error:%v", err)
	}

	r.Header = t.Header
	if len(t.Cookies) > 0 {
		for _, c := range t.Cookies {
			r.AddCookie(c)
		}
	}
	f.dumpRequest(r)

	client := f.httpclient

	if strings.HasPrefix(t.URL, "https") {
		client = f.httpsclient
	}
	client.Timeout = f.timeount
	return client.Do(r)
}

func (f *Fetcher) dumpRequest(req *http.Request) {
	if !f.isdebug {
		return
	}
	dumpReq, _ := httputil.DumpRequest(req, false)
	fmt.Println(string(dumpReq))
}