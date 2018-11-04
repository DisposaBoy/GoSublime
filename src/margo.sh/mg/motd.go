package mg

import (
	"encoding/json"
	"fmt"
	"margo.sh/bolt"
	"margo.sh/mgpf"
	"net/http"
	"net/url"
	"sync"
	"time"
)

var (
	motdK = motdKey{K: "motdState"}
)

type motdAct struct {
	ActionType
	msg string
}

type motdKey struct{ K string }

type motdState struct {
	LastUpdate time.Time
	Result     struct {
		Message string
		Error   string
		ANN     struct {
			Tag struct {
				Y int
				M int
				D int
				N int
			}
			Content string
		}
		Tag string
	}
}

// MOTD keeps you updated about new versions and important announcements
//
// It adds a new command `motd.sync` available via the UserCmd palette as `Sync MOTD`
//
// Interval can be set in order to enable automatic update fetching.
//
// When new updates are found, it displays the message in the status bar
// e.g. `★ margo.sh/cl/18.09.14 ★` a url where you see the upcoming changes before updating
//
// It sends the following data to the url https://api.margo.sh/motd.json:
// * current editor plugin name e.g. `?client=gosublime`
//   this tells us which editor plugin's changelog to check
// * current editor plugin version e.g. `?tag=r18.09.14-1`
//   this allows us to determine if there any updates
// * whether or not this is the first request of the day e.g. `?firstHit=1`
//   this allows us to get an estimated count of active users without storing
//   any personally identifiable data
//
// No other data is sent. For more info contact privacy at kuroku.io
//
type MOTD struct {
	ReducerType

	// Endpoint is the URL to check for new messages
	// By default it's https://api.margo.sh/motd.json
	Endpoint string

	// Interval, if set, specifies how often to automatically fetch messages from Endpoint
	Interval time.Duration

	htc http.Client
	msg string

	mu sync.Mutex
}

func (m *MOTD) RInit(mx *Ctx) {
	if m.Endpoint == "" {
		m.Endpoint = "https://api.margo.sh/motd.json"
	}
}

func (m *MOTD) RCond(mx *Ctx) bool {
	return mx.Editor.Ready()
}

func (m *MOTD) RMount(mx *Ctx) {
	go m.proc(mx)
}

func (m *MOTD) Reduce(mx *Ctx) *State {
	st := mx.State
	switch act := mx.Action.(type) {
	case RunCmd:
		st = st.AddBuiltinCmds(BuiltinCmd{Name: "motd.sync", Run: m.motdSyncCmd})
	case QueryUserCmds:
		st = st.AddUserCmds(UserCmd{Title: "Sync MOTD", Name: "motd.sync"})
	case motdAct:
		m.msg = act.msg
	}
	if m.msg != "" {
		st = st.AddStatus(m.msg)
	}
	return st
}

func (m *MOTD) motdSyncCmd(bx *CmdCtx) *State {
	go func() {
		defer bx.Output.Close()

		err := m.sync(bx.Ctx)
		ms, _ := m.loadState()
		if err != nil {
			fmt.Fprintln(bx.Output, "Error:", err)
		} else {
			fmt.Fprintln(bx.Output, "MOTD:", ms.Result.Message)
			fmt.Fprintln(bx.Output, ms.Result.ANN.Content)
		}
	}()
	return bx.State
}

func (m *MOTD) loadState() (motdState, error) {
	ms := motdState{}
	err := bolt.DS.Load(motdK, &ms)
	return ms, err
}

func (m *MOTD) storeState(ms motdState) error {
	return bolt.DS.Store(motdK, ms)
}

func (m *MOTD) sync(mx *Ctx) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	dest, err := url.Parse(m.Endpoint)
	if err != nil {
		return fmt.Errorf("sync: cannot parse endpoint: %s: %s", m.Endpoint, err)
	}
	qry := dest.Query()
	now := time.Now().UTC()
	qry.Set("client", mx.Editor.Client.Name)
	qry.Set("tag", mx.Editor.Client.Tag)
	curr, _ := m.loadState()
	if layout := "2006-01-02"; curr.LastUpdate.Format(layout) != now.Format(layout) {
		qry.Set("firstHit", "1")
	} else {
		qry.Set("firstHit", "0")
	}
	dest.RawQuery = qry.Encode()

	req, err := http.NewRequest("GET", dest.String(), nil)
	if err != nil {
		return fmt.Errorf("sync: cannot create request: %s", err)
	}
	req.Header.Set("User-Agent", "margo.motd")

	mx.Log.Println("motd: sync: fetching", dest)

	res, err := m.htc.Do(req)
	if err != nil {
		return fmt.Errorf("sync: cannot fetch updates: %s", err)
	}
	defer res.Body.Close()

	next := motdState{LastUpdate: now}
	if err := json.NewDecoder(res.Body).Decode(&next.Result); err != nil {
		return fmt.Errorf("sync: cannot decode request: %s", err)
	}
	m.dispatchMsg(mx, next)

	if err := m.storeState(next); err != nil {
		return fmt.Errorf("sync: cannot store state: %s", err)
	}

	return nil
}

func (m *MOTD) dispatchMsg(mx *Ctx, ms motdState) {
	res := ms.Result
	act := motdAct{}
	ctag := mx.Editor.Client.Tag
	switch {
	case ctag == "":
		mx.Log.Println("motd: client tag is undefined; you might need to restart the editor")
	case res.Tag != ctag:
		act.msg = res.Message
	}
	mx.Store.Dispatch(act)
}

func (m *MOTD) proc(mx *Ctx) {
	printl := func(a ...interface{}) {
		mx.Log.Println(append([]interface{}{"motd:"}, a...)...)
	}

	wait := func(d time.Duration) {
		if d <= time.Second {
			return
		}

		printl("waiting for", mgpf.D(d))
		time.Sleep(d)
	}

	ms, _ := m.loadState()
	m.dispatchMsg(mx, ms)

	iv := m.Interval
	if iv <= 0 {
		return
	}

	if m := 30 * time.Second; iv < m {
		iv = m
	}
	printl("auto-update enabled... checking every", mgpf.D(iv))
	wait(iv - time.Since(ms.LastUpdate))

	for {
		if err := m.sync(mx); err != nil {
			printl(err)
		}
		wait(iv)
	}
}
