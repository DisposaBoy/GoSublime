package mg

type Args struct {
	Store *Store
}

type MargoFunc func(Args)
