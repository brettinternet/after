// Package evidence defines versioned local record contracts, not a trust oracle.
// Validate checks internal consistency; it cannot authenticate an imported record.
package evidence

import "time"

const SchemaVersion = 1

type Digest string

type Completeness string

const (
	Complete   Completeness = "complete"
	Incomplete Completeness = "incomplete"
)

type SourceMode string

const (
	Commit      SourceMode = "commit"
	WorkingTree SourceMode = "working_tree"
	Index       SourceMode = "index"
	MergeBase   SourceMode = "merge_base"
)

type Producer string

const (
	Importer Producer = "importer"
	Runner   Producer = "runner"
)

type Kind string

const (
	Reported   Kind = "reported"
	Observed   Kind = "observed"
	NoEvidence Kind = "none"
)

type Applicability string

const (
	Unknown Applicability = "unknown"
	Current Applicability = "current"
	Stale   Applicability = "stale"
)

type ExecutionOutcome string

const (
	NotRun    ExecutionOutcome = "not_run"
	Completed ExecutionOutcome = "completed"
	Failed    ExecutionOutcome = "failed"
	Cancelled ExecutionOutcome = "cancelled"
)

type ComparisonOutcome string

const (
	NotCompared  ComparisonOutcome = "not_compared"
	Equal        ComparisonOutcome = "equal"
	Different    ComparisonOutcome = "different"
	Incomparable ComparisonOutcome = "incomparable"
	Unstable     ComparisonOutcome = "unstable"
)

type ReportOutcome string

const (
	NoReport   ReportOutcome = "none"
	ReportPass ReportOutcome = "pass"
	ReportFail ReportOutcome = "fail"
	ReportSkip ReportOutcome = "skip"
)

type HumanDecision string

const (
	Pinned   HumanDecision = "pinned"
	Accepted HumanDecision = "accepted"
	Reopened HumanDecision = "reopened"
)

// EvidenceState keeps reported status distinct from AFTER's execution outcome.
// Human decisions deliberately do not live here.
type EvidenceState struct {
	Producer      Producer          `json:"producer"`
	Kind          Kind              `json:"kind"`
	Applicability Applicability     `json:"applicability"`
	Execution     ExecutionOutcome  `json:"execution"`
	Comparison    ComparisonOutcome `json:"comparison"`
	Report        ReportOutcome     `json:"report"`
}

type File struct {
	Path    string `json:"path"`
	Content Digest `json:"content"`
	Mode    string `json:"mode"`
}

type Limitation struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type Snapshot struct {
	SchemaVersion int          `json:"schema_version"`
	ID            Digest       `json:"id"`
	Source        SourceMode   `json:"source"`
	Commit        string       `json:"commit"`
	Unborn        bool         `json:"unborn,omitempty"`
	BaseCommit    string       `json:"base_commit,omitempty"`
	IndexSnapshot Digest       `json:"index_snapshot,omitempty"`
	MergeBase     string       `json:"merge_base,omitempty"`
	Files         []File       `json:"files"`
	Excluded      []Limitation `json:"excluded"`
	Unsupported   []Limitation `json:"unsupported"`
	Completeness  Completeness `json:"completeness"`
	Diff          Digest       `json:"diff"`
	Limits        []string     `json:"limits"`
}

// Scenario points to frozen concrete setup/actions, independently of expectations.
type Scenario struct {
	SchemaVersion int      `json:"schema_version"`
	ID            Digest   `json:"id"`
	Input         Digest   `json:"input"`
	Driver        Digest   `json:"driver"`
	Observer      Digest   `json:"observer"`
	Rules         Digest   `json:"rules"`
	Boundary      string   `json:"boundary"`
	Author        string   `json:"author"`
	Limits        []string `json:"limits"`
}

type SnapshotPair struct {
	Base      Digest `json:"base,omitempty"`
	Candidate Digest `json:"candidate"`
}

type Bindings struct {
	Scenario Digest `json:"scenario"`
	Input    Digest `json:"input"`
	Driver   Digest `json:"driver"`
	Observer Digest `json:"observer"`
	Rules    Digest `json:"rules"`
}

type Environment struct {
	Environment  Digest   `json:"environment"`
	Toolchain    Digest   `json:"toolchain"`
	Dependencies Digest   `json:"dependencies"`
	Argv         []string `json:"argv"`
}

type Artifact struct {
	// Content is the storage key, never a filesystem path.
	Content         Digest       `json:"content"`
	Channel         string       `json:"channel"`
	Bytes           int64        `json:"bytes"`
	MaxBytes        int64        `json:"max_bytes"`
	Completeness    Completeness `json:"completeness"`
	Redacted        bool         `json:"redacted"`
	RedactionPolicy string       `json:"redaction_policy,omitempty"`
	Truncated       bool         `json:"truncated"`
}

type Receipt struct {
	RequestID            Digest        `json:"request_id,omitempty"`
	SchemaVersion        int           `json:"schema_version"`
	ID                   Digest        `json:"id"`
	State                EvidenceState `json:"state"`
	Snapshots            SnapshotPair  `json:"snapshots"`
	Bindings             *Bindings     `json:"bindings,omitempty"`
	BaseEnvironment      *Environment  `json:"base_environment,omitempty"`
	CandidateEnvironment *Environment  `json:"candidate_environment,omitempty"`
	Authorization        Digest        `json:"authorization,omitempty"`
	StartedAt            time.Time     `json:"started_at"`
	FinishedAt           time.Time     `json:"finished_at"`
	Completeness         Completeness  `json:"completeness"`
	Artifacts            []Artifact    `json:"artifacts"`
	Redacted             bool          `json:"redacted,omitempty"`
	RedactionPolicy      string        `json:"redaction_policy,omitempty"`
	Limits               []string      `json:"limits"`
}

type DecisionEvent struct {
	Decision HumanDecision  `json:"decision"`
	At       time.Time      `json:"at"`
	Reason   string         `json:"reason"`
	Review   *ReviewContext `json:"review,omitempty"`
}

// Pin contains no evidence state. Accepting or reopening it cannot change a receipt.
type Pin struct {
	SchemaVersion  int             `json:"schema_version"`
	ID             Digest          `json:"id"`
	Scenario       Digest          `json:"scenario"`
	Expectation    string          `json:"expectation"`
	BasisReceipt   Digest          `json:"basis_receipt"`
	BasisSnapshots SnapshotPair    `json:"basis_snapshots"`
	Decision       HumanDecision   `json:"decision"`
	History        []DecisionEvent `json:"history"`
	Scope          PinScope        `json:"scope,omitempty"`
}
