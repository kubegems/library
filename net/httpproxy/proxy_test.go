package httpproxy

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"
)

func TestClient_RoundTrip(t *testing.T) {
	targetserver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello: " + r.URL.Path))
	}))
	defer targetserver.Close()

	proxyserver := httptest.NewServer(&Server{Prefix: "/v1/proxy"})
	defer proxyserver.Close()

	proxyserverurl, err := url.Parse(proxyserver.URL + "/v1/proxy")
	if err != nil {
		t.Errorf("url.Parse() error = %v", err)
		return
	}

	cli := http.Client{
		Transport: Client{Server: proxyserverurl},
	}
	resp, err := cli.Post(targetserver.URL+"/v1/hello", "text/plain", bytes.NewReader([]byte("hello")))
	if err != nil {
		t.Errorf("Client.Do() error = %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Client.Do() StatusCode = %v", resp.StatusCode)
		return
	}
	dump, err := httputil.DumpResponse(resp, true)
	if err != nil {
		t.Errorf("httputil.DumpResponse() error = %v", err)
		return
	}
	t.Logf("dump = %s", dump)
}
