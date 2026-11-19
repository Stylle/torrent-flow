package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTransmissionWebAndBaseURLs(t *testing.T) {
	for _, v := range []struct{ input, want string }{{"http://nas:9091", "http://nas:9091/transmission/rpc"}, {"http://nas:9091/", "http://nas:9091/transmission/rpc"}, {"http://nas:9091/transmission/web/", "http://nas:9091/transmission/rpc"}, {"http://nas:9091/transmission/web/index.html?lang=zh#home", "http://nas:9091/transmission/rpc"}, {"https://nas/tr/transmission/web/", "https://nas/tr/transmission/rpc"}, {"https://nas/custom-rpc", "https://nas/custom-rpc"}} {
		t.Run(v.input, func(t *testing.T) {
			got := normalizeClient(Client{Kind: "tr", URL: v.input})
			if got.URL != v.want {
				t.Fatal("incorrect RPC normalization")
			}
		})
	}
}
func TestTransmission301PreservesPostAndSession(t *testing.T) {
	redirects, calls := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Error("RPC redirect changed POST to GET")
		}
		user, pw, ok := r.BasicAuth()
		if !ok || user != "fixture-user" || pw != "fixture-password" {
			t.Error("same-origin RPC auth missing")
		}
		if r.URL.Path == "/transmission/rpc" {
			redirects++
			w.Header().Set("Location", "/rpc-final/")
			w.WriteHeader(301)
			return
		}
		if r.URL.Path != "/rpc-final/" {
			t.Error("wrong RPC path")
		}
		var body map[string]any
		if json.NewDecoder(r.Body).Decode(&body) != nil || body["method"] != "torrent-get" {
			t.Error("RPC body lost on redirect")
		}
		if r.Header.Get("X-Transmission-Session-Id") != "fixture-session" {
			w.Header().Set("X-Transmission-Session-Id", "fixture-session")
			w.WriteHeader(409)
			return
		}
		calls++
		fmt.Fprint(w, `{"result":"success","arguments":{"torrents":[]}}`)
	}))
	defer server.Close()
	a := adapter(Client{Kind: "tr", URL: server.URL + "/transmission/web/", User: "fixture-user", Password: "fixture-password"})
	for i := 0; i < 2; i++ {
		if _, e := a.list(context.Background()); e != nil {
			t.Fatal(e)
		}
	}
	if redirects != 1 || calls != 2 {
		t.Fatal("RPC redirected endpoint/session was not cached")
	}
}
func TestTransmissionRedirectNeverForwardsCredentialsToAnotherPort(t *testing.T) {
	received := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { received = true; fmt.Fprint(w, `{}`) }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", target.URL+"/rpc")
		w.WriteHeader(301)
	}))
	defer server.Close()
	a := adapter(Client{Kind: "tr", URL: server.URL, User: "fixture", Password: "fixture"})
	_, err := a.list(context.Background())
	if err == nil || !strings.Contains(err.Error(), "another host/port") || received {
		t.Fatal("cross-origin RPC credential redirect was not blocked")
	}
}
func TestTransmissionRedirectLoopAndMissingSessionHeader(t *testing.T) {
	for _, status := range []int{301, 409} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if status == 301 {
					w.Header().Set("Location", "/transmission/rpc")
				}
				w.WriteHeader(status)
			}))
			defer server.Close()
			a := adapter(Client{Kind: "tr", URL: server.URL})
			_, e := a.list(context.Background())
			if e == nil {
				t.Fatal("invalid RPC negotiation was accepted")
			}
		})
	}
}
