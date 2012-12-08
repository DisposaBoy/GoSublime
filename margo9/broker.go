package main

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
	Data  interface{} `json:"data"`
}

type Broker struct {
	served uint
	start  time.Time
	rLck   sync.Mutex
	wLck   sync.Mutex
	r      io.Reader
	w      io.Writer
	in     *bufio.Reader
	out    *json.Encoder
	Wg     *sync.WaitGroup
}

func NewBroker(r io.Reader, w io.Writer) *Broker {
	return &Broker{
		r:   r,
		w:   w,
		in:  bufio.NewReader(r),
		out: json.NewEncoder(w),
		Wg:  &sync.WaitGroup{},
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
	b.wLck.Lock()
	defer b.wLck.Unlock()

	if resp.Data == nil {
		resp.Data = M{}
	}

	s, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	// the only expected write failure are due to broken pipes
	// which usually means the client has gone away so just ignore the error
	b.w.Write(s)
	b.w.Write([]byte{'\n'})
	return nil
}

func (b *Broker) call(req *Request, cl Caller) {
	defer b.Wg.Done()
	b.served++

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

func (b *Broker) accept() (stopLooping bool) {
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

	b.Wg.Add(1)
	go b.call(req, cl)

	return
}

func (b *Broker) Loop(decorate bool) {
	b.start = time.Now()

	if decorate {
		go b.SendNoLog(Response{
			Token: "margo.hello",
			Data: M{
				"time": b.start.String(),
			},
		})
	}

	for {
		stopLooping := b.accept()
		if stopLooping {
			break
		}
		runtime.Gosched()
	}

	if decorate {
		b.SendNoLog(Response{
			Token: "margo.bye-ni",
			Data: M{
				"served": b.served,
				"uptime": time.Now().Sub(b.start).String(),
			},
		})
	}
}
