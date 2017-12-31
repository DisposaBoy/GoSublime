package margo_pkg

type mHello M

func (m mHello) Call() (interface{}, string) {
	return m, ""
}

func init() {
	registry.Register("hello", func(_ *Broker) Caller {
		return &mHello{}
	})
}
