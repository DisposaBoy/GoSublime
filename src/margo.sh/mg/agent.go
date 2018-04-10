package mg

import (
	"bufio"
	"fmt"
	"github.com/ugorji/go/codec"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
)

var (
	// DefaultCodec is the name of the default codec used for IPC communication
	DefaultCodec = "json"

	// codecHandles is the map of all valid codec handles
	codecHandles = func() map[string]codec.Handle {
		m := map[string]codec.Handle{
			"cbor": &codec.CborHandle{},
			"json": &codec.JsonHandle{
				Indent:         2,
				TermWhitespace: true,
			},
			"msgpack": &codec.MsgpackHandle{},
		}
		m[""] = m[DefaultCodec]
		return m
	}()

	// CodecNames is the list of names of all valid codec handles
	CodecNames = func() []string {
		l := make([]string, 0, len(codecHandles))
		for k, _ := range codecHandles {
			if k != "" {
				l = append(l, k)
			}
		}
		sort.Strings(l)
		return l
	}()

	// CodecNamesStr is the list of names of all valid codec handles in the form `a, b or c`
	CodecNamesStr = func() string {
		i := len(CodecNames) - 1
		return strings.Join(CodecNames[:i], ", ") + " or " + CodecNames[i]

	}()
)

type AgentConfig struct {
	// the name of the agent as used in the command `margo.sh [start...] $AgentName`
	AgentName string

	// Codec is the name of the codec to use for IPC
	// Valid values are json, cbor or msgpack
	// Default: json
	Codec string

	Stdin  io.ReadCloser
	Stdout io.WriteCloser
	Stderr io.WriteCloser
}

type agentReqAction struct {
	Name string
	Data codec.Raw
}

type agentReq struct {
	Cookie  string
	Actions []agentReqAction
	Props   clientProps
}

func newAgentReq() *agentReq {
	return &agentReq{Props: makeClientProps()}
}

func (rq *agentReq) finalize(ag *Agent) {
	rq.Props.finalize(ag)
}

type agentRes struct {
	Cookie string
	Error  string
	State  *State
}

func (rs agentRes) finalize() interface{} {
	out := struct {
		agentRes
		State struct {
			State
			Config        interface{}
			ClientActions []clientActionType
		}
	}{}
	out.agentRes = rs
	out.State.State = *rs.State
	inSt := &out.State.State
	outSt := &out.State

	outSt.ClientActions = inSt.clientActions

	if out.Error == "" {
		out.Error = strings.Join([]string(outSt.Errors), "\n")
	}

	if outSt.View.changed == 0 {
		outSt.View = nil
	}

	if ec := inSt.Config; ec != nil {
		outSt.Config = ec.EditorConfig()
	}

	return out
}

type Agent struct {
	Name string

	Log   *Logger
	Store *Store

	mu sync.Mutex

	stdin  io.ReadCloser
	stdout io.WriteCloser
	stderr io.WriteCloser

	handle codec.Handle
	enc    *codec.Encoder
	encWr  *bufio.Writer
	dec    *codec.Decoder
}

func (ag *Agent) Run() error {
	defer ag.shutdownIPC()
	return ag.communicate()
}

func (ag *Agent) communicate() error {
	ag.Log.Println("started")
	ag.Store.dispatch(Started{})
	ag.Store.ready()

	for {
		rq := newAgentReq()
		if err := ag.dec.Decode(rq); err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("ipc.decode: %s", err)
		}
		rq.finalize(ag)
		// TODO: put this on a channel in the future.
		// at the moment we lock the store and block new requests to maintain request/response order
		// but decoding time could become a problem if we start sending large requests from the client
		// we currently only have 1 client (GoSublime) that we also control so it's ok for now...
		ag.Store.syncRq(ag, rq)
	}
	return nil
}

func (ag *Agent) createAction(ra agentReqAction, h codec.Handle) (Action, error) {
	if f := actionCreators[ra.Name]; f != nil {
		return f(h, ra)
	}
	return nil, fmt.Errorf("Unknown action: %s", ra.Name)
}

func (ag *Agent) listener(st *State) {
	err := ag.send(agentRes{State: st})
	if err != nil {
		ag.Log.Println("agent.send failed. shutting down ipc:", err)
		go ag.shutdownIPC()
	}
}

func (ag *Agent) send(res agentRes) error {
	ag.mu.Lock()
	defer ag.mu.Unlock()

	defer ag.encWr.Flush()
	return ag.enc.Encode(res.finalize())
}

func (ag *Agent) shutdownIPC() {
	defer ag.stdin.Close()
	defer ag.stdout.Close()
}

func NewAgent(cfg AgentConfig) (*Agent, error) {
	ag := &Agent{
		Name:   cfg.AgentName,
		stdin:  cfg.Stdin,
		stdout: cfg.Stdout,
		stderr: cfg.Stderr,
		handle: codecHandles[cfg.Codec],
	}
	if ag.stdin == nil {
		ag.stdin = os.Stdin
	}
	if ag.stdout == nil {
		ag.stdout = os.Stdout
	}
	if ag.stderr == nil {
		ag.stderr = os.Stderr
	}
	ag.stdin = &LockedReadCloser{ReadCloser: ag.stdin}
	ag.stdout = &LockedWriteCloser{WriteCloser: ag.stdout}
	ag.stderr = &LockedWriteCloser{WriteCloser: ag.stderr}
	ag.Log = NewLogger(ag.stderr)
	ag.Store = newStore(ag, ag.listener).
		Before(defaultReducers.before...).
		Use(defaultReducers.use...).
		After(defaultReducers.after...)

	if e := os.Getenv("MARGO_BUILD_ERROR"); e != "" {
		ag.Store.Use(Reduce(func(mx *Ctx) *State {
			return mx.AddStatus(e)
		}))
	}

	if ag.handle == nil {
		return ag, fmt.Errorf("Invalid codec '%s'. Expected %s", cfg.Codec, CodecNamesStr)
	}
	ag.encWr = bufio.NewWriter(ag.stdout)
	ag.enc = codec.NewEncoder(ag.encWr, ag.handle)
	ag.dec = codec.NewDecoder(bufio.NewReader(ag.stdin), ag.handle)

	return ag, nil
}

func (ag *Agent) Args() Args {
	return Args{
		Store: ag.Store,
		Log:   ag.Log,
	}
}
