// Package analytics parses Caddy JSON access logs and ingests them into SQLite.
package analytics

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"strings"
	"time"

	"github.com/schmorrison/goshpanel/internal/store"
)

// caddyAccessLog is the subset of Caddy's JSON access log we care about.
type caddyAccessLog struct {
	TS      float64 `json:"ts"`
	Status  int     `json:"status"`
	Size    int64   `json:"size"`
	Request struct {
		Method   string `json:"method"`
		URI      string `json:"uri"`
		Host     string `json:"host"`
		RemoteIP string `json:"remote_ip"`
	} `json:"request"`
}

// ParseLine parses one Caddy JSON access log line.
func ParseLine(line string) (store.AccessLogEvent, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return store.AccessLogEvent{}, false
	}
	var entry caddyAccessLog
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		return store.AccessLogEvent{}, false
	}
	path := entry.Request.URI
	if i := strings.Index(path, "?"); i >= 0 {
		path = path[:i]
	}
	ts := time.Now().UTC()
	if entry.TS > 0 {
		sec := int64(entry.TS)
		nsec := int64((entry.TS - float64(sec)) * float64(time.Second))
		ts = time.Unix(sec, nsec).UTC()
	}
	return store.AccessLogEvent{
		RecordedAt: ts,
		Host:       entry.Request.Host,
		Method:     entry.Request.Method,
		Path:       path,
		Status:     entry.Status,
		Bytes:      entry.Size,
		RemoteIP:   entry.Request.RemoteIP,
	}, true
}

// IngestFile reads new lines from path starting at offset, stores events, returns new offset.
func IngestFile(st *store.Store, path string) (int64, error) {
	offset, err := st.AnalyticsOffset()
	if err != nil {
		return 0, err
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return offset, nil
		}
		return offset, err
	}
	defer f.Close()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return offset, err
	}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var batch []store.AccessLogEvent
	for sc.Scan() {
		if ev, ok := ParseLine(sc.Text()); ok {
			batch = append(batch, ev)
		}
		if len(batch) >= 200 {
			if err := st.RecordAccessLogEvents(batch); err != nil {
				return offset, err
			}
			batch = batch[:0]
		}
	}
	if len(batch) > 0 {
		if err := st.RecordAccessLogEvents(batch); err != nil {
			return offset, err
		}
	}
	if err := sc.Err(); err != nil {
		return offset, err
	}
	newOffset, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return offset, err
	}
	if err := st.SetAnalyticsOffset(newOffset); err != nil {
		return offset, err
	}
	return newOffset, nil
}
