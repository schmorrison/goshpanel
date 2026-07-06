package email

import (
	"fmt"
	"strings"

	"github.com/schmorrison/goshpanel/internal/store"
)

// DeliverabilityRecord is a suggested DNS TXT/MX record for mail authentication.
type DeliverabilityRecord struct {
	Type     string
	Name     string
	Value    string
	Priority int
	Label    string
}

// SuggestDeliverability returns SPF, DMARC, and DKIM placeholder records for a domain.
func SuggestDeliverability(domain, serverIP, dkimSelector string) []DeliverabilityRecord {
	domain = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domain)), ".")
	if dkimSelector == "" {
		dkimSelector = "goshpanel"
	}
	var out []DeliverabilityRecord
	spf := "v=spf1 mx"
	if serverIP != "" {
		spf += " ip4:" + strings.TrimSpace(serverIP)
	}
	spf += " -all"
	out = append(out, DeliverabilityRecord{
		Type: "TXT", Name: "@", Value: spf, Label: "SPF",
	})
	out = append(out, DeliverabilityRecord{
		Type: "TXT", Name: "_dmarc", Value: fmt.Sprintf("v=DMARC1; p=quarantine; rua=mailto:dmarc@%s", domain), Label: "DMARC",
	})
	out = append(out, DeliverabilityRecord{
		Type: "TXT", Name: dkimSelector + "._domainkey",
		Value: fmt.Sprintf("v=DKIM1; k=rsa; p=REPLACE_WITH_YOUR_DKIM_PUBLIC_KEY; /* generate with: openssl genrsa -out dkim.pem 2048 */"),
		Label: "DKIM (placeholder — replace public key)",
	})
	return out
}

// ApplyDeliverability inserts suggested records that are not already present.
func ApplyDeliverability(st *store.Store, domainID int64, domainName, serverIP, dkimSelector string) (int, error) {
	existing, err := st.DNSRecords(domainID)
	if err != nil {
		return 0, err
	}
	has := func(name, typ, value string) bool {
		for _, r := range existing {
			if strings.EqualFold(r.Name, name) && strings.EqualFold(r.Type, typ) && r.Value == value {
				return true
			}
		}
		return false
	}
	added := 0
	for _, rec := range SuggestDeliverability(domainName, serverIP, dkimSelector) {
		if has(rec.Name, rec.Type, rec.Value) {
			continue
		}
		if _, err := st.CreateDNSRecord(store.DNSRecord{
			DomainID: domainID, Type: rec.Type, Name: rec.Name, Value: rec.Value, TTL: 3600, Priority: rec.Priority,
		}); err != nil {
			return added, err
		}
		added++
	}
	return added, nil
}
