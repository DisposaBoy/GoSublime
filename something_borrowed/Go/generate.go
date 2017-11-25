//go:generate go run generate.go

// we want to re-use the existing Go syntax,
// but AFAIK, there's no way to do that if we disable the Go package...
// and we/I don't want to enable it because it includes a lot annoying snippets, etc.
package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
)

func main() {
	urls := map[string]string{
		"Comments.tmPreferences":          "https://raw.githubusercontent.com/sublimehq/Packages/master/Go/Comments.tmPreferences",
		"Indentation Rules.tmPreferences": "https://raw.githubusercontent.com/sublimehq/Packages/master/Go/Indentation%20Rules.tmPreferences",
		"Go.sublime-syntax":               "https://raw.githubusercontent.com/sublimehq/Packages/master/Go/Go.sublime-syntax",
	}
	for name, url := range urls {
		dl(name, url)
	}

	cmd := exec.Command("git", "status", ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func dl(name, url string) {
	fmt.Printf("Sync %s: ", name)

	resp, err := http.Get(url)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return
	}

	ioutil.WriteFile(name, content, 0644)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("ok")
}
