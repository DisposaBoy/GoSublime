package main

import (
	"time"
)

type mPing struct {
	Delay time.Duration `json:"delay"`
}

func (m *mPing) Call() (interface{}, string) {
	start := time.Now()
	time.Sleep(m.Delay * time.Millisecond)
	return M{
		"start": start.String(),
		"end":   time.Now().String(),
	}, ""
}

func init() {
	registry.Register("ping", func(_ *Broker) Caller {
		return &mPing{}
	})
}
