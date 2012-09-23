package main

import (
	"encoding/json"
	"os"
	"runtime"
)

func main() {
	root := os.Getenv("GOROOT")
	if root == "" {
		root = runtime.GOROOT()
	}
	m := map[string]string{
		"GOROOT": root,
		"GOPATH": os.Getenv("GOPATH"),
		"GOBIN":  os.Getenv("GOBIN"),
		"GOARCH": runtime.GOARCH,
		"GOOS":   runtime.GOOS,
	}
	json.NewEncoder(os.Stdout).Encode(m)
}
