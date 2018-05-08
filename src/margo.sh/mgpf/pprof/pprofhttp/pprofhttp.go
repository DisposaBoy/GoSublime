package pprofhttp

import (
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"strings"
)

func ListenAndServe(addr string) (net.Listener, error) {
	if strings.HasPrefix(addr, ":") {
		addr = "localhost" + addr
	}

	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	go http.Serve(l, nil)
	return l, nil
}

func listen(host string, ports []string) (net.Listener, error) {
	var l net.Listener
	var err error
	for _, p := range ports {
		l, err = net.Listen("tcp", host+":"+p)
		if err == nil {
			return l, nil
		}
	}
	return nil, err
}

func StartServer(logger *log.Logger) {
	host := "localhost"
	l, err := listen(host, []string{"10101", "20202", "30303", "0"})
	if err != nil {
		logger.Println("pprof listen failed:", err)
		return
	}

	laddr := l.Addr().String()
	_, port, err := net.SplitHostPort(laddr)
	if err == nil {
		laddr = host + ":" + port
	}
	logger.Println("pprof addr:", "http://"+laddr)
	logger.Println("pprof access:", "`go tool pprof http://"+laddr+"/debug/pprof/profile`")

	go func() {
		defer l.Close()

		err := http.Serve(l, nil)
		if err != nil {
			logger.Println("pprof serve failed:", err)
		}
	}()
}
