package main

import (
	"os"
)

func main() {
	defer os.Stdin.Close()
	defer os.Stdout.Close()
	defer os.Stderr.Close()

	broker := NewBroker(os.Stdin, os.Stdout)
	broker.Loop()
}
