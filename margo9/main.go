package main

import (
	"flag"
	"io"
	"os"
	"strings"
)

func main() {
	do := flag.String("do", "-", "Process the specified operations(lines) operation and exit. `-` means operate as normal")
	flag.Parse()

	defer os.Stdin.Close()
	defer os.Stdout.Close()
	defer os.Stderr.Close()

	var in io.Reader = os.Stdin
	doCall := *do != "-"
	if doCall {
		in = strings.NewReader(*do)
	}

	broker := NewBroker(in, os.Stdout)
	broker.Loop(!doCall)
}
