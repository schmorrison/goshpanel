package analytics

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/schmorrison/goshpanel/internal/store"
)

func TestParseLine(t *testing.T) {
	line := `{"level":"info","ts":1710000000.5,"status":200,"size":1234,"request":{"method":"GET","uri":"/index.html?x=1","host":"example.com","remote_ip":"203.0.113.1"}}`
	ev, ok := ParseLine(line)
	if !ok {
		t.Fatal("expected parse ok")
	}
	if ev.Method != "GET" || ev.Path != "/index.html" || ev.Status != 200 || ev.Bytes != 1234 {
		t.Errorf("event = %+v", ev)
	}
	if ev.Host != "example.com" || ev.RemoteIP != "203.0.113.1" {
		t.Errorf("host/ip = %+v", ev)
	}
	if ev.RecordedAt.Before(time.Unix(1710000000, 0)) {
		t.Errorf("time = %v", ev.RecordedAt)
	}
}

func TestParseLineInvalid(t *testing.T) {
	if _, ok := ParseLine("not json"); ok {
		t.Fatal("expected false")
	}
	if _, ok := ParseLine(""); ok {
		t.Fatal("expected false")
	}
}

func TestIngestFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/access.log"
	content := strings.Join([]string{
		`{"ts":1710000001,"status":200,"size":100,"request":{"method":"GET","uri":"/a","host":"h","remote_ip":"1.1.1.1"}}`,
		`{"ts":1710000002,"status":404,"size":50,"request":{"method":"GET","uri":"/b","host":"h","remote_ip":"1.1.1.1"}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(dir + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := IngestFile(st, path); err != nil {
		t.Fatal(err)
	}
	sum, err := st.AccessLogSummarySince(time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if sum.TotalRequests != 2 || sum.Status2xx != 1 || sum.Status4xx != 1 {
		t.Errorf("summary = %+v", sum)
	}
}
