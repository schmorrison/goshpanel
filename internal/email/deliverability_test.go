package email

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/schmorrison/goshpanel/internal/store"
)

func TestSuggestDeliverability(t *testing.T) {
	recs := SuggestDeliverability("example.com", "203.0.113.1", "goshpanel")
	if len(recs) != 3 {
		t.Fatalf("got %d records", len(recs))
	}
	spf := recs[0]
	if spf.Type != "TXT" || !strings.Contains(spf.Value, "v=spf1") || !strings.Contains(spf.Value, "203.0.113.1") {
		t.Errorf("spf = %+v", spf)
	}
	if recs[1].Name != "_dmarc" || !strings.Contains(recs[1].Value, "DMARC1") {
		t.Errorf("dmarc = %+v", recs[1])
	}
	if !strings.Contains(recs[2].Name, "_domainkey") {
		t.Errorf("dkim = %+v", recs[2])
	}
}

func TestApplyDeliverability(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	d, err := st.CreateDomain("mail.example.com", "/srv/mail", "")
	if err != nil {
		t.Fatal(err)
	}
	added, err := ApplyDeliverability(st, d.ID, d.Name, "203.0.113.1", "")
	if err != nil {
		t.Fatal(err)
	}
	if added != 3 {
		t.Errorf("added = %d, want 3", added)
	}
	added2, err := ApplyDeliverability(st, d.ID, d.Name, "203.0.113.1", "")
	if err != nil {
		t.Fatal(err)
	}
	if added2 != 0 {
		t.Errorf("second apply added = %d, want 0", added2)
	}
}
