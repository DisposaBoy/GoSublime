package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"log"
	"runtime"
	"sync"
	"time"
)

type M map[string]interface{}

type Request struct {
	Method string
	Token  string
	data   []byte
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
	wg     *sync.WaitGroup
	r      io.Reader
	w      io.Writer
	in     *bufio.Reader
	out    *json.Encoder
}

func NewBroker(r io.Reader, w io.Writer) *Broker {
	return &Broker{
		wg:  &sync.WaitGroup{},
		r:   r,
		w:   w,
		in:  bufio.NewReader(r),
		out: json.NewEncoder(w),
	}
}

func (b *Broker) Recv() (*Request, error) {
	b.rLck.Lock()
	defer b.rLck.Unlock()

	req := &Request{}
	line, readErr := b.in.ReadBytes('\n')

	// EOF is not an unexpected error
	if readErr != nil && readErr != io.EOF {
		return req, readErr
	}

	i := bytes.IndexByte(line, '\t')
	if i >= 0 {
		req.data = line[i:]
		line = line[:i]
	}

	// ignore empty lines
	line = bytes.TrimSpace(line)
	if len(line) > 0 {
		decErr := json.Unmarshal(line, req)
		if decErr != nil {
			return req, decErr
		}
	}

	// return readErr so we can pass back a possible EOF
	return req, readErr
}

func (b *Broker) Send(resp Response) error {
	b.wLck.Lock()
	defer b.wLck.Unlock()

	if resp.Data == nil {
		resp.Data = M{}
	}

	err := b.out.Encode(resp)
	if err != nil {
		log.Println("broker: Cannot send result", err)
	}
	return err
}

func (b *Broker) call(req *Request, cl Caller) {
	defer b.wg.Done()
	b.served++

	defer func() {
		err := recover()
		if err != nil {
			log.Printf("broker: %v#%v PANIC: %v\n", req.Method, req.Token, err)
			b.Send(Response{
				Token: req.Token,
				Error: "broker: " + req.Method + "#" + req.Token + " PANIC",
			})
		}
	}()

	res, err := cl.Call()
	b.Send(Response{
		Token: req.Token,
		Error: err,
		Data:  res,
	})
}

func (b *Broker) accept() (stopLooping bool) {
	req, err := b.Recv()
	if err != nil {
		// try to handle the last request before returning
		if req != nil && err == io.EOF {
			stopLooping = true
		} else {
			log.Println("broker: Cannot read input", err)
			b.Send(Response{
				Token: req.Token,
				Error: err.Error(),
			})
			return true
		}
	}

	if req.Method == "" {
		return
	}

	if req.Method == "bye-ni" {
		return true
	}

	m := registry.Lookup(req.Method)
	if m == nil {
		e := "Invalid method " + req.Method
		log.Println("broker:", e)
		b.Send(Response{
			Token: req.Token,
			Error: e,
		})
		return
	}

	cl := m()
	if ar, ok := cl.(Arger); ok {
		err = json.Unmarshal(req.data, ar.Arg())
		if err != nil {
			log.Println("broker: Cannot decode arg", err)
			b.Send(Response{
				Token: req.Token,
				Error: err.Error(),
			})
			return
		}
	}

	b.wg.Add(1)
	go b.call(req, cl)

	return
}

func (b *Broker) Loop() {
	b.start = time.Now()

	go b.Send(Response{
		Token: "margo.hello",
		Data: M{
			"time": b.start.String(),
		},
	})

	for {
		stopLooping := b.accept()
		if stopLooping {
			break
		}
		runtime.Gosched()
	}

	b.wg.Wait()
	b.Send(Response{
		Token: "margo.bye-ni",
		Data: M{
			"served": b.served,
			"uptime": time.Now().Sub(b.start).String(),
		},
	})
}
