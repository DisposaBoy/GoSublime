package main

import (
	"bytes"
	"encoding/base64"
	"flag"
	"io"
	"os"
	"strings"
)

func main() {
	do := "-"
	flag.StringVar(&do, "do", "-", "Process the specified operations(lines) operation and exit. `-` means operate as normal")
	flag.Parse()

	defer os.Stdin.Close()
	defer os.Stdout.Close()
	defer os.Stderr.Close()

	var in io.Reader = os.Stdin
	doCall := do != "-"
	if doCall {
		b64 := "base64:"
		if strings.HasPrefix(do, b64) {
			s, _ := base64.StdEncoding.DecodeString(do[len(b64):])
			in = bytes.NewReader(s)
		} else {
			in = strings.NewReader(do)
		}
	}

	broker := NewBroker(in, os.Stdout)
	broker.Loop(!doCall)
}
