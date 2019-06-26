package vfs

import (
	"log"
	"os"
	"path/filepath"
	"testing"
)

var testPath = func() string {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalln("Getwd:", err)
	}
	// true story...
	base := "node_modules/babel-preset-react-app/node_modules/@babel/preset-env/node_modules/@babel/plugin-proposal-unicode-property-regex/test/fixtures/without-unicode-flag/script-extensions"
	return filepath.Join(wd, filepath.FromSlash(base))
}()

func TestNodePath(t *testing.T) {
	nd := New().Poke(testPath)
	if p := nd.Path(); p != testPath {
		t.Fatalf("Node.Path() returned `%s`. Expected `%s`", p, testPath)
	}
}

func BenchmarkPath(b *testing.B) {
	nd := New().Poke(testPath)
	for i := 0; i < b.N; i++ {
		nd.Path()
	}
}

func TestPeekPoke(t *testing.T) {
	fs := New()
	if fs.Peek(testPath) != nil {
		t.Fatal("Peek of non-existent node should return nil")
	}
	nd := fs.Poke(testPath)
	if nd == nil {
		t.Fatal("Poke returned nil")
	}
	if fs.Peek(testPath) != nd {
		t.Fatalf("Peek failed to find poked node %p", fs.Peek(testPath))
	}
	if fs.Poke(testPath) != nd {
		t.Fatal("Poke of existing path returned a differnt node")
	}
}

func BenchmarkPeek(b *testing.B) {
	fs := New()
	fs.Poke(testPath)
	for i := 0; i < b.N; i++ {
		fs.Peek(testPath)
	}
}

func BenchmarkPoke(b *testing.B) {
	b.Run("Miss", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			fs := New()
			fs.Poke(testPath)
		}
	})
	b.Run("Hit", func(b *testing.B) {
		fs := New()
		fs.Poke(testPath)
		for i := 0; i < b.N; i++ {
			fs.Poke(testPath)
		}
	})
}
