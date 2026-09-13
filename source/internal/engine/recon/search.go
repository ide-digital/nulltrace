package recon

import (
	"github.com/4x3/nulltrace/internal/broker"
	"github.com/4x3/nulltrace/internal/engine/mailer"
)

func MapBrokers(id mailer.IdentityView, brokers []broker.Broker) []Finding {
	if len(brokers) == 0 {
		return nil
	}
	out := make([]Finding, 0, len(brokers))
	for _, b := range brokers {
		if b.ID == "haveibeenpwned" {
			continue
		}
		f := Score(id, b)
		if f.BrokerID == "" || f.Confidence <= 0 {
			continue
		}
		out = append(out, f)
	}
	return out
}
