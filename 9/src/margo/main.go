package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

type NoInputErr string

func (s NoInputErr) Error() string {
	return string(s)
}

var (
	actions    = map[string]Action{}
	acLck      = sync.Mutex{}
	acQuitting = false
	acWg       = sync.WaitGroup{}
	acListener net.Listener
)

func act(ac Action) {
	ac.Path = normPath(ac.Path)
	acLck.Lock()
	defer acLck.Unlock()
	if _, exists := actions[ac.Path]; exists {
		log.Fatalf("Action exists: %s\n", ac.Path)
	}
	if ac.Func == nil {
		log.Fatalf("Invalid action: %s\n", ac.Path)
	}
	actions[ac.Path] = ac
}

func normPath(p string) string {
	return path.Clean("/" + strings.ToLower(strings.TrimSpace(p)))
}

type data interface{}

type Response struct {
	Error string `json:"error"`
	Data  data   `json:"data"`
}

type Request struct {
	Rw  http.ResponseWriter
	Req *http.Request
}

func (r Request) Decode(a interface{}) error {
	data := []byte(r.Req.FormValue("data"))
	if len(data) == 0 {
		return NoInputErr("Data is empty")
	}
	return json.Unmarshal(data, a)
}

func parseAstFile(fn string, s string, mode parser.Mode) (fset *token.FileSet, af *ast.File, err error) {
	fset = token.NewFileSet()
	var src interface{}
	if s != "" {
		src = s
	}
	if fn == "" {
		fn = "<stdin>"
	}
	af, err = parser.ParseFile(fset, fn, src, mode)
	return
}

type ActionFunc func(r Request) (data, error)

type Action struct {
	Path string
	Doc  string
	Func ActionFunc
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func callAction(ac Action, r Request) (res data, err error) {
	defer func() {
		if e := recover(); e != nil {
			res = nil
			err = errors.New(fmt.Sprintf("margo%s panic: %s", ac.Path, e))
		}
	}()
	res, err = ac.Func(r)
	return
}

func serve(rw http.ResponseWriter, req *http.Request) {
	acWg.Add(1)
	defer acWg.Done()

	rw.Header().Set("Content-Type", "application/json; charset=utf-8")
	r := Request{
		Rw:  rw,
		Req: req,
	}
	path := normPath(req.URL.Path)
	resp := Response{}

	defer func() {
		json.NewEncoder(rw).Encode(resp)
	}()

	if ac, ok := actions[path]; ok {
		var err error
		resp.Data, err = callAction(ac, r)
		if err != nil {
			resp.Error = err.Error()
		}
	} else {
		resp.Error = "Invalid action: " + path
	}
}

func sendQuit(addr string) {
	if resp, err := http.Get(`http://` + addr + `/?data="bye%20ni"`); err == nil {
		resp.Body.Close()
	}
}

func newPrinter(tabIndent bool, tabWidth int) *printer.Config {
	mode := printer.UseSpaces
	if tabIndent {
		mode |= printer.TabIndent
	}
	return &printer.Config{
		Mode:     mode,
		Tabwidth: tabWidth,
	}
}

func printSrc(fset *token.FileSet, v interface{}, tabIndent bool, tabWidth int) (src string, err error) {
	p := newPrinter(tabIndent, tabWidth)
	buf := &bytes.Buffer{}
	if err = p.Fprint(buf, fset, v); err == nil {
		src = buf.String()
	}
	return
}

func rootDirs(env map[string]string) []string {
	dirs := []string{}
	gopath := ""
	if len(env) == 0 {
		gopath = os.Getenv("GOPATH")
	} else {
		gopath = env["GOPATH"]
	}

	gorootBase := runtime.GOROOT()
	if len(env) > 0 && env["GOROOT"] != "" {
		gorootBase = env["GOROOT"]
	} else if fn := os.Getenv("GOROOT"); fn != "" {
		gorootBase = fn
	}
	goroot := filepath.Join(gorootBase, "src", "pkg")

	dirsSeen := map[string]bool{}
	for _, fn := range filepath.SplitList(gopath) {
		if dirsSeen[fn] {
			continue
		}
		dirsSeen[fn] = true

		// goroot may be a part of gopath and we don't want that
		if fn != "" && !strings.HasPrefix(fn, gorootBase) {
			fn := filepath.Join(fn, "src")
			if fi, err := os.Stat(fn); err == nil && fi.IsDir() {
				dirs = append(dirs, fn)
			}
		}
	}

	if fi, err := os.Stat(goroot); err == nil && fi.IsDir() {
		dirs = append(dirs, goroot)
	}

	return dirs
}

func fiHasGoExt(fi os.FileInfo) bool {
	return strings.HasSuffix(fi.Name(), ".go")
}

func isGoFile(fi os.FileInfo) bool {
	fn := fi.Name()
	return fn[0] != '.' && fn[0] != '_' && strings.HasSuffix(fn, ".go")
}

func parsePkg(fset *token.FileSet, srcDir string, mode parser.Mode) (pkg *ast.Package, pkgs map[string]*ast.Package, err error) {
	if pkgs, err = parser.ParseDir(fset, srcDir, fiHasGoExt, mode); pkgs != nil {
		_, pkgName := filepath.Split(srcDir)
		// we aren't going to support package whose name don't match the directory unless it's main
		p, ok := pkgs[pkgName]
		if !ok {
			p, ok = pkgs["main"]
		}
		if ok {
			pkg, err = ast.NewPackage(fset, p.Files, nil, nil)
		}
	}
	return
}

func findPkg(fset *token.FileSet, importPath string, dirs []string, mode parser.Mode) (pkg *ast.Package, pkgs map[string]*ast.Package, err error) {
	for _, dir := range dirs {
		srcDir := filepath.Join(dir, importPath)
		if pkg, pkgs, err = parsePkg(fset, srcDir, mode); pkg != nil {
			return
		}
	}
	return
}

func main() {
	defaultAddr := "127.0.0.1:57951"

	d := flag.Bool("d", false, "Whether or not to launch in the background(like a daemon)")
	closeFds := flag.Bool("close-fds", false, "Whether or not to close stdin, stdout and stderr")
	addr := flag.String("addr", defaultAddr, "The tcp address to listen on")
	call := flag.String("call", "",
		"Call the specified command:"+
			"\n\t\tdefault-addr: output the default address"+
			"\n\t\tquit:         send a quit signal to *addr* (equivalent to the GET request: http://*addr*/?data=\"bye ni\")"+
			"\n\t\treplace:      send a quit signal to *addr* then startup as normal"+
			"")
	flag.Parse()

	switch *call {
	case "":
		// startup as normal
	case "quit":
		sendQuit(*addr)
		return
	case "replace":
		// handled below
	case "default-addr":
		fmt.Println(defaultAddr)
		return
	default:
		log.Fatalf("invalid call: expected one of `quit, replace, default-addr', got `%s'\n", *call)
	}

	if *d {
		cmd := exec.Command(os.Args[0],
			"-close-fds",
			"-addr", *addr,
			"-call", *call,
		)
		serr, err := cmd.StderrPipe()
		if err != nil {
			log.Fatalln(err)
		}
		err = cmd.Start()
		if err != nil {
			log.Fatalln(err)
		}
		s, err := ioutil.ReadAll(serr)
		s = bytes.TrimSpace(s)
		if bytes.HasPrefix(s, []byte("addr: ")) {
			fmt.Println(string(s))
			cmd.Process.Release()
		} else {
			log.Printf("unexpected response from MarGo: `%s` error: `%v`\n", s, err)
			cmd.Process.Kill()
		}
	} else {
		if *call == "replace" {
			sendQuit(*addr)
		}

		var err error
		acListener, err = net.Listen("tcp", *addr)
		if err != nil {
			log.Fatalln(err)
		}

		fmt.Fprintf(os.Stderr, "addr: http://%s\n", acListener.Addr())
		if *closeFds {
			os.Stdin.Close()
			os.Stdout.Close()
			os.Stderr.Close()
		}

		go func() {
			importPaths(map[string]string{})
			pkgDirs(nil)
		}()

		err = http.Serve(acListener, http.HandlerFunc(serve))
		if !acQuitting && err != nil {
			log.Fatalln(err)
		}
		acWg.Wait()
	}
}
