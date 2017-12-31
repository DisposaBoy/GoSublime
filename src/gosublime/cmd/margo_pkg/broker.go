package margo_pkg

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"runtime"
	"sync"
	"time"
)

type M map[string]interface{}

type Request struct {
	Method string
	Token  string
}

type Response struct {
	Token string      `json:"token"`
	Error string      `json:"error"`
	Tag   string      `json:"tag"`
	Data  interface{} `json:"data"`
}

type Job struct {
	Req *Request
	Cl  Caller
}

type Broker struct {
	sync.Mutex

	tag    string
	served counter
	start  time.Time
	r      io.Reader
	w      io.Writer
	in     *bufio.Reader
	out    *json.Encoder
}

func NewBroker(r io.Reader, w io.Writer, tag string) *Broker {
	return &Broker{
		tag: tag,
		r:   r,
		w:   w,
		in:  bufio.NewReader(r),
		out: json.NewEncoder(w),
	}
}

func (b *Broker) Send(resp Response) error {
	err := b.SendNoLog(resp)
	if err != nil {
		logger.Println("Cannot send result", err)
	}
	return err
}

func (b *Broker) SendNoLog(resp Response) error {
	b.Lock()
	defer b.Unlock()

	if resp.Data == nil {
		resp.Data = M{}
	}

	if resp.Tag == "" {
		resp.Tag = b.tag
	}

	s, err := json.Marshal(resp)
	if err != nil {
		// if there is a token, it means the client is waiting for a response
		// so respond with the json error. cause of json encode failure includes: non-utf8 string
		if resp.Token == "" {
			return err
		}

		s, err = json.Marshal(M{
			"error": "margo broker: cannot encode json response: " + err.Error(),
		})
		if err != nil {
			return err
		}
	}

	// the only expected write failure are due to broken pipes
	// which usually means the client has gone away so just ignore the error
	b.w.Write(s)
	b.w.Write([]byte{'\n'})
	return nil
}

func (b *Broker) call(req *Request, cl Caller) {
	b.served.next()

	defer func() {
		err := recover()
		if err != nil {
			buf := make([]byte, 64*1024*1024)
			n := runtime.Stack(buf, true)
			logger.Printf("%v#%v PANIC: %v\n%s\n\n", req.Method, req.Token, err, buf[:n])
			b.Send(Response{
				Token: req.Token,
				Error: "broker: " + req.Method + "#" + req.Token + " PANIC",
			})
		}
	}()

	res, err := cl.Call()
	if res == nil {
		res = M{}
	} else if v, ok := res.(M); ok && v == nil {
		res = M{}
	}

	b.Send(Response{
		Token: req.Token,
		Error: err,
		Data:  res,
	})
}

func (b *Broker) accept(jobsCh chan Job) (stopLooping bool) {
	line, err := b.in.ReadBytes('\n')

	if err == io.EOF {
		stopLooping = true
	} else if err != nil {
		logger.Println("Cannot read input", err)
		b.Send(Response{
			Error: err.Error(),
		})
		return
	}

	req := &Request{}
	dec := json.NewDecoder(bytes.NewReader(line))
	// if this fails, we are unable to return a useful error(no token to send it to)
	// so we'll simply/implicitly drop the request since it has no method
	// we can safely assume that all such cases will be empty lines and not an actual request
	dec.Decode(&req)

	if req.Method == "" {
		return
	}

	if req.Method == "bye-ni" {
		return true
	}

	m := registry.Lookup(req.Method)
	if m == nil {
		e := "Invalid method " + req.Method
		logger.Println(e)
		b.Send(Response{
			Token: req.Token,
			Error: e,
		})
		return
	}

	cl := m(b)
	err = dec.Decode(cl)
	if err != nil {
		logger.Println("Cannot decode arg", err)
		b.Send(Response{
			Token: req.Token,
			Error: err.Error(),
		})
		return
	}

	jobsCh <- Job{
		Req: req,
		Cl:  cl,
	}

	return
}

func (b *Broker) worker(wg *sync.WaitGroup, jobsCh chan Job) {
	defer wg.Done()
	for job := range jobsCh {
		b.call(job.Req, job.Cl)
	}
}

func (b *Broker) Loop(decorate bool, wait bool) {
	b.start = time.Now()

	if decorate {
		go b.SendNoLog(Response{
			Token: "margo.hello",
			Data: M{
				"time": b.start.String(),
			},
		})
	}

	const workers = 20
	wg := &sync.WaitGroup{}
	jobsCh := make(chan Job, 1000)
	for i := 0; i < workers; i += 1 {
		wg.Add(1)
		go b.worker(wg, jobsCh)
	}

	for {
		stopLooping := b.accept(jobsCh)
		if stopLooping {
			break
		}
		runtime.Gosched()
	}
	close(jobsCh)

	if wait {
		wg.Wait()
	}

	if decorate {
		b.SendNoLog(Response{
			Token: "margo.bye-ni",
			Data: M{
				"served": b.served.val(),
				"uptime": time.Now().Sub(b.start).String(),
			},
		})
	}
}
