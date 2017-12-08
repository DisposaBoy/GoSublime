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
	"path/filepath"
)

type dlFile struct {
	name string
	url  string
	dirs []string
}

func main() {
	urls := []dlFile{
		{
			name: "Comments.tmPreferences",
			url:  "https://raw.githubusercontent.com/sublimehq/Packages/master/Go/Comments.tmPreferences",
			dirs: []string{"../..", "."},
		},
		{
			name: "Indentation Rules.tmPreferences",
			url:  "https://raw.githubusercontent.com/sublimehq/Packages/master/Go/Indentation%20Rules.tmPreferences",
			dirs: []string{"../..", "."},
		},
		{
			name: "Go.sublime-syntax",
			url:  "https://raw.githubusercontent.com/sublimehq/Packages/master/Go/Go.sublime-syntax",
			dirs: []string{"."},
		},
	}
	for _, f := range urls {
		dl(f)
	}

	cmd := exec.Command("git", "status", "--short")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func dl(f dlFile) {
	fmt.Printf("Sync %s: ", f.name)

	resp, err := http.Get(f.url)
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

	for _, dir := range f.dirs {
		ioutil.WriteFile(filepath.Join(dir, f.name), content, 0644)
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	fmt.Println("ok")
}
