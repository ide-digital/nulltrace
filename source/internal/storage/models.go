package storage

import (
	"time"
)

type IdentityRow struct {
	ID            string
	FirstNameEnc  []byte
	LastNameEnc   []byte
	MiddleNameEnc []byte
	DOBEnc        []byte
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type AttributeRow struct {
	ID         string
	IdentityID string
	Type       string
	ValueEnc   []byte
	IsPrimary  bool
	CreatedAt  time.Time
}

type BrokerRow struct {
	ID                        string
	Name                      string
	Domain                    string
	Category                  string
	Mechanism                 string
	OptOutURL                 string
	ContactEmail              string
	RequiresCaptcha           bool
	RequiresEmailConfirmation bool
	JurisdictionCoverage      string
	RepopulationPeriodDays    int
	Notes                     string
	Playbook                  string
}

type ExposedRecordRow struct {
	ID              string
	IdentityID      string
	BrokerID        string
	ProfileURL      string
	ExtractedEnc    []byte
	Confidence      float64
	Status          string
	RiskTier        string
	FirstDetectedAt time.Time
	LastVerifiedAt  time.Time
}

type ActionRow struct {
	ID                string
	ExposedRecordID   string
	BrokerID          string
	Strategy          string
	CurrentState      string
	Attempts          int
	TrackingToken     string
	RequestPayloadEnc []byte
	ResponseLogEnc    []byte
	StatutoryDeadline *time.Time
	CompletedAt       *time.Time
	LastError         string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type AuditRow struct {
	ID          int64
	EventUUID   string
	Timestamp   time.Time
	EventType   string
	BrokerID    string
	PayloadHash string
	DetailsEnc  []byte
}

type DashboardStats struct {
	BrokerCount     int
	ActiveRemovals  int
	VerifiedRemoved int
	ExposedRecords  int
	FailedActions   int
	AwaitingConfirm int
	ManualPending   int
	Overdue         int
}
