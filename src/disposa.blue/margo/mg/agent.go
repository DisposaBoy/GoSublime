package mg

import (
	"bufio"
	"fmt"
	"github.com/ugorji/go/codec"
	"io"
	"log"
	"os"
	"sort"
	"strings"
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
	// Codec is the name of the codec to use for IPC
	// Valid values are json, cbor or msgpack
	// Default: json
	Codec string
}

type agentRequest struct {
	Cookie string
	Action struct {
		Name string
		Data codec.Raw
	}
}

type agentResponse struct {
	Cookie string
	Error  string
	State  State
}

type Agent struct {
	*log.Logger
	*Store

	stdin  io.ReadCloser
	stdout io.WriteCloser
	stderr io.WriteCloser

	handle codec.Handle
	enc    *codec.Encoder
	dec    *codec.Decoder
}

func (ag *Agent) Run() error {
	defer ag.stdin.Close()
	defer ag.stdout.Close()
	return ag.communicate()
}

func (ag *Agent) sync(req agentRequest) {
	ag.syncState(req)
	ag.syncAction(req)
}

func (ag *Agent) syncState(req agentRequest) {
}

func (ag *Agent) syncAction(req agentRequest) {
	name := req.Action.Name
	data := req.Action.Data
	a := createAction(name)

	if a == nil {
		ag.Println("unknown action:", name)
		return
	}

	if len(data) != 0 {
		err := codec.NewDecoderBytes(data, ag.handle).Decode(a)
		if err != nil {
			ag.Printf("cannot decode action: %s: %s\n", name, err)
			return
		}
	}

	ag.Dispatch(a)
}

func (ag *Agent) communicate() error {
	ag.Subscribe(func(st State) { ag.send(st) })
	ag.Println("ready")
	ag.Dispatch(StartAction{})

	for {
		req := agentRequest{}
		if err := ag.dec.Decode(&req); err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("ipc.decode: %s", err)
		}

		ag.sync(req)
	}
	return nil
}

func (ag *Agent) send(s State) error {
	return ag.enc.Encode(agentResponse{
		State: s,
	})
}

func NewAgent(cfg AgentConfig) (*Agent, error) {
	ag := &Agent{
		Logger: log.New(os.Stderr, "margo@", log.Lshortfile|log.Ltime),
		Store:  NewStore(),
		stdin:  os.Stdin,
		stdout: os.Stdout,
		stderr: os.Stderr,
		handle: codecHandles[cfg.Codec],
	}

	if ag.handle == nil {
		return ag, fmt.Errorf("Invalid codec '%s'. Expected %s", cfg.Codec, CodecNamesStr)
	}
	ag.enc = codec.NewEncoder(bufio.NewWriter(ag.stdout), ag.handle)
	ag.dec = codec.NewDecoder(bufio.NewReader(ag.stdin), ag.handle)

	return ag, nil
}
