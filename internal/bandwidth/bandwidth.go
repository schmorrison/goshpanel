package bandwidth

import (
	"bufio"
	"encoding/json"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/schmorrison/goshpanel/internal/store"
)

// HostStats summarizes traffic for one hostname.
type HostStats struct {
	Host     string
	Requests int64
	Bytes    int64
}

// TopHosts returns the busiest hosts from Caddy JSON access log events.
func TopHosts(events []store.AccessLogEvent, limit int) []HostStats {
	counts := map[string]*HostStats{}
	for _, e := range events {
		h := strings.ToLower(strings.TrimSpace(e.Host))
		if h == "" {
			continue
		}
		if counts[h] == nil {
			counts[h] = &HostStats{Host: h}
		}
		counts[h].Requests++
		counts[h].Bytes += e.Bytes
	}
	out := make([]HostStats, 0, len(counts))
	for _, s := range counts {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Bytes == out[j].Bytes {
			return out[i].Requests > out[j].Requests
		}
		return out[i].Bytes > out[j].Bytes
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// HourlyBuckets builds request counts per hour for the last N hours.
func HourlyBuckets(events []store.AccessLogEvent, hours int) []int64 {
	if hours <= 0 {
		hours = 24
	}
	now := time.Now().UTC().Truncate(time.Hour)
	buckets := make([]int64, hours)
	for _, e := range events {
		t := e.RecordedAt.UTC().Truncate(time.Hour)
		diff := int(now.Sub(t).Hours())
		if diff < 0 || diff >= hours {
			continue
		}
		idx := hours - 1 - diff
		buckets[idx]++
	}
	return buckets
}

// ParseCaddyJSONLine ingests one Caddy JSON log line into an event.
func ParseCaddyJSONLine(line string) (store.AccessLogEvent, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return store.AccessLogEvent{}, false
	}
	var raw struct {
		TS     json.RawMessage `json:"ts"`
		Request struct {
			Host   string `json:"host"`
			Method string `json:"method"`
			URI    string `json:"uri"`
			Remote struct {
				IP string `json:"ip"`
			} `json:"remote"`
		} `json:"request"`
		Status int   `json:"status"`
		Size   int64 `json:"size"`
	}
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return store.AccessLogEvent{}, false
	}
	ts := time.Now().UTC()
	return store.AccessLogEvent{
		RecordedAt: ts,
		Host:       raw.Request.Host,
		Method:     raw.Request.Method,
		Path:       raw.Request.URI,
		Status:     raw.Status,
		Bytes:      raw.Size,
		RemoteIP:   raw.Request.Remote.IP,
	}, true
}

// TailFile reads up to maxLines from the end of a log file.
func TailFile(path string, maxLines int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
		if len(lines) > maxLines*2 {
			lines = lines[len(lines)-maxLines:]
		}
	}
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return lines, sc.Err()
}
