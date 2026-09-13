package broker

import "strings"

type Category string

const (
	CategoryPeopleSearch        Category = "PEOPLE_SEARCH"
	CategoryWholesaleAggregator Category = "WHOLESALE_AGGREGATOR"
	CategoryFinancial           Category = "FINANCIAL"
	CategoryMarketing           Category = "MARKETING"
	CategoryPeopleLocator       Category = "PEOPLE_LOCATOR"
	CategoryBackground          Category = "BACKGROUND"
)

type Mechanism string

const (
	MechanismWebForm    Mechanism = "WEB_FORM"
	MechanismEmailLegal Mechanism = "EMAIL_LEGAL"
	MechanismAPI        Mechanism = "API"
	MechanismHybrid     Mechanism = "HYBRID"
)

type Jurisdiction string

const (
	JurisdictionAll  Jurisdiction = "ALL"
	JurisdictionCCPA Jurisdiction = "CCPA_ONLY"
	JurisdictionGDPR Jurisdiction = "GDPR_ONLY"
	JurisdictionUS   Jurisdiction = "US_STATE"
)

type RecordStatus string

const (
	StatusDiscovered      RecordStatus = "DISCOVERED"
	StatusApproved        RecordStatus = "APPROVED_FOR_SCRUB"
	StatusIgnored         RecordStatus = "IGNORED"
	StatusVerifiedRemoved RecordStatus = "VERIFIED_REMOVED"
	StatusPendingReview   RecordStatus = "PENDING_REVIEW"
)

type ActionState string

const (
	StateQueued               ActionState = "QUEUED"
	StateSubmitted            ActionState = "SUBMITTED"
	StateAwaitingConfirmation ActionState = "AWAITING_CONFIRMATION"
	StateAwaitingManual       ActionState = "AWAITING_MANUAL"
	StateCompleted            ActionState = "COMPLETED"
	StateFailed               ActionState = "FAILED"
	StateRejected             ActionState = "REJECTED"
)

type Strategy string

const (
	StrategySMTP    Strategy = "SMTP_DEMAND"
	StrategyManual  Strategy = "MANUAL_WEB_FORM"
	StrategyBrowser Strategy = "BROWSER_AUTOMATION"
)

type RiskTier string

const (
	RiskCritical RiskTier = "CRITICAL"
	RiskHigh     RiskTier = "HIGH"
	RiskMedium   RiskTier = "MEDIUM"
	RiskLow      RiskTier = "LOW"
)

type AttributeType string

const (
	AttrEmail    AttributeType = "EMAIL"
	AttrPhone    AttributeType = "PHONE"
	AttrAddress  AttributeType = "ADDRESS"
	AttrUsername AttributeType = "USERNAME"
	AttrRelative AttributeType = "RELATIVE"
	AttrAlias    AttributeType = "ALIAS"
	AttrCity     AttributeType = "CITY"
)

type Broker struct {
	ID                        string       `yaml:"id"`
	Name                      string       `yaml:"name"`
	Domain                    string       `yaml:"domain"`
	Category                  Category     `yaml:"category"`
	Mechanism                 Mechanism    `yaml:"mechanism"`
	OptOutURL                 string       `yaml:"opt_out_url"`
	ContactEmail              string       `yaml:"contact_email"`
	RequiresCaptcha           bool         `yaml:"requires_captcha"`
	RequiresEmailConfirmation bool         `yaml:"requires_email_confirmation"`
	JurisdictionCoverage      Jurisdiction `yaml:"jurisdiction_coverage"`
	RepopulationPeriodDays    int          `yaml:"repopulation_period_days"`
	Notes                     string       `yaml:"notes"`
	Playbook                  string       `yaml:"playbook"`
}

func (b Broker) Slug() string {
	if b.ID != "" {
		return strings.ToLower(b.ID)
	}
	return strings.ToLower(strings.ReplaceAll(b.Domain, ".", "-"))
}

func (b Broker) DeadlineDays() int {
	return 30
}

func (b Broker) CanAutoEmail() bool {
	return b.ContactEmail != "" && (b.Mechanism == MechanismEmailLegal || b.Mechanism == MechanismHybrid)
}

func (b Broker) RequiresHuman() bool {
	return b.Mechanism == MechanismWebForm || b.RequiresCaptcha
}

func ParseMechanism(s string) Mechanism {
	switch Mechanism(strings.ToUpper(s)) {
	case MechanismWebForm, MechanismEmailLegal, MechanismAPI, MechanismHybrid:
		return Mechanism(strings.ToUpper(s))
	default:
		return MechanismEmailLegal
	}
}

func ParseCategory(s string) Category {
	c := Category(strings.ToUpper(s))
	switch c {
	case CategoryPeopleSearch, CategoryWholesaleAggregator, CategoryFinancial, CategoryMarketing, CategoryPeopleLocator, CategoryBackground:
		return c
	default:
		return CategoryPeopleSearch
	}
}

func ParseJurisdiction(s string) Jurisdiction {
	j := Jurisdiction(strings.ToUpper(s))
	switch j {
	case JurisdictionAll, JurisdictionCCPA, JurisdictionGDPR, JurisdictionUS:
		return j
	default:
		return JurisdictionAll
	}
}
