package mg

type Args struct {
	*Store
	Log *Logger
}

type MargoFunc func(Args)
