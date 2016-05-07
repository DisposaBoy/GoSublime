package main

import (
	"bytes"
	"io/ioutil"
	"net/http"
)

type mShare struct {
	Src string
}

func (m mShare) Call() (interface{}, string) {
	res := M{}

	s := bytes.TrimSpace([]byte(m.Src))
	if len(s) == 0 {
		return res, "Nothing to share"
	}

	u := "http://play.golang.org"
	body := bytes.NewBufferString(m.Src)
	req, err := http.NewRequest("POST", u+"/share", body)
	req.Header.Set("User-Agent", "GoSublime")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return res, err.Error()
	}
	defer resp.Body.Close()

	s, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return res, err.Error()
	}

	res["url"] = u + "/p/" + string(s)
	e := ""
	if resp.StatusCode != 200 {
		e = "Unexpected http status: " + resp.Status
	}

	return res, e
}

func init() {
	registry.Register("share", func(_ *Broker) Caller {
		return &mShare{}
	})
}
