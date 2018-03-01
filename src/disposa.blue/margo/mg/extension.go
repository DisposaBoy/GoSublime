package mg

type Args struct {
	Store *Store
	Log   *Logger
}

type MargoFunc func(Args)
