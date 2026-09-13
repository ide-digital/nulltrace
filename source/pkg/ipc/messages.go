package ipc

import "encoding/json"

const ProtocolVersion = 1

const (
	CmdPing           = "ping"
	CmdStatus         = "status"
	CmdSnapshot       = "snapshot"
	CmdScan           = "scan"
	CmdScrub          = "scrub"
	CmdExport         = "export"
	CmdActionComplete = "action_complete"
	CmdActionVerify   = "action_verify"
	CmdIdentityAdd    = "identity_add"
	CmdAttrAdd        = "attr_add"
	CmdSecretSet      = "secret_set"
	CmdShutdown       = "shutdown"
	CmdPlaybook       = "playbook"
	CmdFootprint      = "footprint"
	CmdAutomate       = "automate"
)

type Envelope struct {
	V       int             `json:"v"`
	ID      string          `json:"id"`
	Token   string          `json:"token,omitempty"`
	Cmd     string          `json:"cmd"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type Reply struct {
	V       int             `json:"v"`
	ID      string          `json:"id"`
	OK      bool            `json:"ok"`
	Error   string          `json:"error,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type StatusPayload struct {
	Unlocked        bool   `json:"unlocked"`
	ListenAddr      string `json:"listen_addr"`
	BrokerCount     int    `json:"broker_count"`
	ActiveRemovals  int    `json:"active_removals"`
	VerifiedRemoved int    `json:"verified_removed"`
	ExposedRecords  int    `json:"exposed_records"`
	FailedActions   int    `json:"failed_actions"`
	AwaitingConfirm int    `json:"awaiting_confirm"`
	ManualPending   int    `json:"manual_pending"`
	Overdue         int    `json:"overdue"`
	IdentityName    string `json:"identity_name,omitempty"`
}

type ScanRequest struct {
	EnableHIBP bool `json:"enable_hibp"`
}

type ScanResult struct {
	FindingCount int `json:"finding_count"`
	BreachCount  int `json:"breach_count"`
	Persisted    int `json:"persisted"`
}

type ScrubRequest struct {
	All bool `json:"all"`
}

type ScrubResult struct {
	QueuedSMTP   int `json:"queued_smtp"`
	QueuedManual int `json:"queued_manual"`
	Skipped      int `json:"skipped"`
}

type ExportRequest struct {
	Format string `json:"format"` // md | json | csv
}

type ExportResult struct {
	Path string `json:"path"`
}

type ActionIDRequest struct {
	ActionID string `json:"action_id"`
	RecordID string `json:"record_id,omitempty"`
}

type IdentityAddRequest struct {
	First  string   `json:"first"`
	Last   string   `json:"last"`
	Middle string   `json:"middle"`
	DOB    string   `json:"dob"`
	Emails []string `json:"emails"`
	Phones []string `json:"phones"`
	Cities []string `json:"cities"`
}

type IdentityAddResult struct {
	ID string `json:"id"`
}

type AttrAddRequest struct {
	IdentityID string `json:"identity_id"`
	Type       string `json:"type"`
	Value      string `json:"value"`
	Primary    bool   `json:"primary"`
}

type SecretSetRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type PlaybookRequest struct {
	BrokerID string `json:"broker_id"`
}

type PlaybookResult struct {
	Text string `json:"text"`
}

type FootprintRequest struct {
	Username string `json:"username"`
	Persist  bool   `json:"persist"`
}

type FootprintSite struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type FootprintResult struct {
	Username string          `json:"username"`
	Checked  int             `json:"checked"`
	Found    int             `json:"found"`
	Results  []FootprintSite `json:"results"`
}

type AutomateRequest struct {
	BrokerID string `json:"broker_id"`
	Headful  bool   `json:"headful"`
}

type AutomateResult struct {
	BrokerID   string `json:"broker_id"`
	BrokerName string `json:"broker_name"`
	Status     string `json:"status"`
	Detail     string `json:"detail"`
	URL        string `json:"url"`
}

type ExposedView struct {
	ID             string  `json:"id"`
	BrokerID       string  `json:"broker_id"`
	BrokerName     string  `json:"broker_name"`
	Domain         string  `json:"domain,omitempty"`
	Category       string  `json:"category,omitempty"`
	Mechanism      string  `json:"mechanism,omitempty"`
	Status         string  `json:"status"`
	RiskTier       string  `json:"risk_tier"`
	Confidence     float64 `json:"confidence"`
	ProfileURL     string  `json:"profile_url"`
	OptOutURL      string  `json:"opt_out_url,omitempty"`
	ContactEmail   string  `json:"contact_email,omitempty"`
	Notes          string  `json:"notes,omitempty"`
	Detected       string  `json:"detected"`
	LastVerified   string  `json:"last_verified,omitempty"`
	ActionState    string  `json:"action_state,omitempty"`
	ActionStrategy string  `json:"action_strategy,omitempty"`
}

type ActionView struct {
	ID         string `json:"id"`
	RecordID   string `json:"record_id,omitempty"`
	BrokerID   string `json:"broker_id"`
	BrokerName string `json:"broker_name"`
	Strategy   string `json:"strategy"`
	State      string `json:"state"`
	Attempts   int    `json:"attempts"`
	Tracking   string `json:"tracking"`
	LastError  string `json:"last_error"`
	Deadline   string `json:"deadline,omitempty"`
	Updated    string `json:"updated"`
}

type AuditView struct {
	Timestamp string `json:"timestamp"`
	EventType string `json:"event_type"`
	BrokerID  string `json:"broker_id"`
	Details   string `json:"details"`
}

type IdentityView struct {
	ID        string   `json:"id"`
	First     string   `json:"first"`
	Middle    string   `json:"middle"`
	Last      string   `json:"last"`
	DOB       string   `json:"dob"`
	Emails    []string `json:"emails"`
	Phones    []string `json:"phones"`
	Addresses []string `json:"addresses"`
	Cities    []string `json:"cities"`
	Usernames []string `json:"usernames"`
}

type Snapshot struct {
	Status    StatusPayload `json:"status"`
	Identity  IdentityView  `json:"identity"`
	Exposed   []ExposedView `json:"exposed"`
	Actions   []ActionView  `json:"actions"`
	Audit     []AuditView   `json:"audit"`
	Playbooks []string      `json:"playbooks"`
}
