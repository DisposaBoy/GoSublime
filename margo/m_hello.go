package main

type mHello struct {
	S string `json:"s"`
}

func (h *mHello) Call() (interface{}, string) {
	return h, ""
}

func init() {
	registry.Register("hello", func(_ *Broker) Caller {
		return &mHello{}
	})
}
