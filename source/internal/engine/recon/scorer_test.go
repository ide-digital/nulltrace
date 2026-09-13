package recon

import (
	"testing"

	"github.com/4x3/nulltrace/internal/broker"
	"github.com/4x3/nulltrace/internal/engine/mailer"
)

func TestScoreNameOnlyIsLow(t *testing.T) {
	id := mailer.IdentityView{First: "Ada", Last: "Lovelace"}
	b := broker.Broker{ID: "spokeo", Name: "Spokeo", Category: broker.CategoryPeopleSearch}
	f := Score(id, b)
	if f.Confidence > 0.30 {
		t.Fatalf("name-only confidence %v want <= 0.30", f.Confidence)
	}
	if f.RiskTier != string(broker.RiskLow) {
		t.Fatalf("risk %s want LOW", f.RiskTier)
	}
}

func TestScoreEmailPhoneCityIsHigh(t *testing.T) {
	id := mailer.IdentityView{
		First:     "Ada",
		Last:      "Lovelace",
		Emails:    []string{"ada@example.com"},
		Phones:    []string{"+1 555 0100"},
		Cities:    []string{"London"},
		Addresses: []string{"1 Example St"},
	}
	b := broker.Broker{ID: "spokeo", Name: "Spokeo", Category: broker.CategoryPeopleSearch}
	f := Score(id, b)
	if f.Confidence < 0.70 {
		t.Fatalf("joint identifiers confidence %v want >= 0.70", f.Confidence)
	}
	if f.RiskTier != string(broker.RiskCritical) && f.RiskTier != string(broker.RiskHigh) {
		t.Fatalf("risk %s want HIGH/CRITICAL", f.RiskTier)
	}
}

func TestScoreEmptyIdentitySkipped(t *testing.T) {
	f := Score(mailer.IdentityView{}, broker.Broker{ID: "x", Category: broker.CategoryMarketing})
	if f.BrokerID != "" {
		t.Fatalf("empty identity should not score")
	}
}

func TestMapBrokersSkipsHIBP(t *testing.T) {
	id := mailer.IdentityView{First: "A", Last: "B", Emails: []string{"a@b.c"}}
	out := MapBrokers(id, []broker.Broker{
		{ID: "haveibeenpwned", Name: "HIBP", Category: broker.CategoryBackground, Mechanism: broker.MechanismAPI},
		{ID: "acxiom", Name: "Acxiom", Category: broker.CategoryWholesaleAggregator},
	})
	if len(out) != 1 || out[0].BrokerID != "acxiom" {
		t.Fatalf("got %#v", out)
	}
}

func TestBreachTier(t *testing.T) {
	if g := breachTier([]string{"Passwords", "Email addresses"}); g != "CRITICAL" {
		t.Fatalf("got %s", g)
	}
	if g := breachTier([]string{"Phone numbers"}); g != "HIGH" {
		t.Fatalf("got %s", g)
	}
	if g := breachTier([]string{"Usernames"}); g != "MEDIUM" {
		t.Fatalf("got %s", g)
	}
}
