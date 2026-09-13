package recon

import (
	"fmt"
	"strings"

	"github.com/4x3/nulltrace/internal/broker"
	"github.com/4x3/nulltrace/internal/engine/mailer"
)

func Score(id mailer.IdentityView, b broker.Broker) Finding {
	hasName := strings.TrimSpace(id.First) != "" && strings.TrimSpace(id.Last) != ""
	hasEmail := len(nonEmpty(id.Emails)) > 0
	hasPhone := len(nonEmpty(id.Phones)) > 0
	hasCity := len(nonEmpty(id.Cities)) > 0 || len(nonEmpty(id.Addresses)) > 0
	hasUser := len(nonEmpty(id.Usernames)) > 0 || len(nonEmpty(id.Aliases)) > 0

	if !hasName && !hasEmail && !hasPhone && !hasCity && !hasUser {
		return Finding{}
	}

	var conf float64
	var reasons []string

	switch b.Category {
	case broker.CategoryPeopleSearch, broker.CategoryPeopleLocator, broker.CategoryBackground:
		if hasName {
			conf += 0.22
			reasons = append(reasons, "legal name matches the class of identifier these listings are built from")
		}
		if hasCity {
			conf += 0.18
			reasons = append(reasons, "city/address present")
		}
		if hasPhone {
			conf += 0.20
			reasons = append(reasons, "phone present")
		}
		if hasEmail {
			conf += 0.12
			reasons = append(reasons, "email present")
		}
		if hasUser {
			conf += 0.08
			reasons = append(reasons, "username/alias present")
		}
	case broker.CategoryMarketing, broker.CategoryWholesaleAggregator:
		if hasEmail {
			conf += 0.38
			reasons = append(reasons, "email is the primary key for marketing files")
		}
		if hasPhone {
			conf += 0.22
			reasons = append(reasons, "phone present")
		}
		if hasName {
			conf += 0.10
			reasons = append(reasons, "name present")
		}
		if hasCity {
			conf += 0.10
			reasons = append(reasons, "location present")
		}
	case broker.CategoryFinancial:
		if hasName {
			conf += 0.20
			reasons = append(reasons, "name present")
		}
		if hasCity {
			conf += 0.22
			reasons = append(reasons, "address-class identifier present")
		}
		if hasEmail {
			conf += 0.12
		}
		if hasPhone {
			conf += 0.12
		}
	default:
		if hasName {
			conf += 0.15
		}
		if hasEmail {
			conf += 0.15
		}
	}

	if hasEmail && hasPhone && hasCity {
		conf += 0.14
		reasons = append(reasons, "email+phone+city jointly declared")
	} else if hasName && hasPhone && hasCity {
		conf += 0.10
		reasons = append(reasons, "name+phone+city jointly declared")
	}

	if conf > 0.95 {
		conf = 0.95
	}

	if hasName && !hasEmail && !hasPhone && !hasCity && !hasUser {
		if conf > 0.28 {
			conf = 0.28
		}
		if conf < 0.18 {
			conf = 0.18
		}
	}

	if conf <= 0 {
		return Finding{}
	}

	reason := fmt.Sprintf("heuristic map onto %s (%s): %s", b.Name, b.Category, strings.Join(reasons, "; "))
	return Finding{
		BrokerID:   b.ID,
		BrokerName: b.Name,
		Reason:     reason,
		Confidence: conf,
		RiskTier:   tierFromConfidence(conf, b.Category),
		ProfileURL: b.OptOutURL,
	}
}

func tierFromConfidence(conf float64, cat broker.Category) string {
	switch {
	case conf >= 0.80:
		if cat == broker.CategoryPeopleSearch || cat == broker.CategoryBackground {
			return string(broker.RiskCritical)
		}
		return string(broker.RiskHigh)
	case conf >= 0.55:
		return string(broker.RiskHigh)
	case conf >= 0.32:
		return string(broker.RiskMedium)
	default:
		return string(broker.RiskLow)
	}
}

func nonEmpty(ss []string) []string {
	var out []string
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}
