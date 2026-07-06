package httptool

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExecute(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer srv.Close()
	res := Execute(context.Background(), Request{Method: "GET", URL: srv.URL})
	if res.Error != "" || res.StatusCode != 200 || res.Body != "ok" {
		t.Errorf("res = %+v", res)
	}
}

func TestParseHeaders(t *testing.T) {
	h := ParseHeaders("X-Test: one\nAuthorization: Bearer abc")
	if h["X-Test"] != "one" || h["Authorization"] != "Bearer abc" {
		t.Errorf("headers = %v", h)
	}
}
