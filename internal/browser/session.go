package browser

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"path"
	"strings"

	"github.com/brettinternet/after/internal/capture"
	"github.com/brettinternet/after/internal/evidence"
)

const ReviewSessionVersion = 1

// ReviewSession is bounded UI state, not an evidence record. It contains only
// immutable snapshot IDs, the comparison mode, and the options for a future
// safe Git capture.
type ReviewSession struct {
	SchemaVersion int                   `json:"schema_version"`
	Pair          evidence.SnapshotPair `json:"pair"`
	Mode          evidence.ReviewMode   `json:"mode"`
	Capture       CaptureFlags          `json:"capture"`
}

// CaptureFlags is the serializable portion of capture.Options. It stores no
// captured source bytes or derived inventory.
type CaptureFlags struct {
	Staged           bool     `json:"staged,omitempty"`
	Base             string   `json:"base,omitempty"`
	Target           string   `json:"target,omitempty"`
	IncludeUntracked []string `json:"include_untracked,omitempty"`
}

func NewReviewSession(pair evidence.SnapshotPair, options capture.Options) ReviewSession {
	flags := CaptureFlags{IncludeUntracked: append([]string(nil), options.IncludeUntracked...)}
	switch options.Mode {
	case evidence.Index:
		flags.Staged = true
	case evidence.MergeBase:
		flags.Base, flags.Target = options.Base, options.Target
	}
	return ReviewSession{SchemaVersion: ReviewSessionVersion, Pair: pair, Mode: evidence.OriginalBase, Capture: flags}
}

func (s ReviewSession) CaptureOptions() capture.Options {
	options := capture.Options{IncludeUntracked: append([]string(nil), s.Capture.IncludeUntracked...)}
	switch {
	case s.Capture.Staged:
		options.Mode = evidence.Index
	case s.Capture.Base != "":
		options.Mode = evidence.MergeBase
		options.Base, options.Target = s.Capture.Base, s.Capture.Target
	default:
		options.Mode = evidence.WorkingTree
	}
	return options
}

func (s ReviewSession) Validate() error {
	if s.SchemaVersion != ReviewSessionVersion || !validSessionDigest(s.Pair.Base) || !validSessionDigest(s.Pair.Candidate) || s.Mode != evidence.OriginalBase {
		return errors.New("invalid version, snapshot IDs, or comparison mode")
	}
	if (s.Capture.Staged && (s.Capture.Base != "" || s.Capture.Target != "")) || ((s.Capture.Base == "") != (s.Capture.Target == "")) {
		return errors.New("invalid capture flag combination")
	}
	if len(s.Capture.Base) > 4096 || len(s.Capture.Target) > 4096 || strings.ContainsRune(s.Capture.Base, 0) || strings.ContainsRune(s.Capture.Target, 0) {
		return errors.New("capture refs exceed bounds")
	}
	if len(s.Capture.IncludeUntracked) > 128 {
		return errors.New("too many selected untracked paths")
	}
	seen := map[string]bool{}
	for _, name := range s.Capture.IncludeUntracked {
		if len(name) > 4096 || name == "" || name == "." || name == ".." || path.IsAbs(name) || path.Clean(name) != name || strings.ContainsAny(name, "\\\x00:") || seen[name] {
			return errors.New("invalid selected untracked path")
		}
		for _, segment := range strings.Split(name, "/") {
			if strings.EqualFold(segment, ".after") || strings.EqualFold(segment, ".git") {
				return errors.New("capture path selects private storage")
			}
		}
		seen[name] = true
	}
	return nil
}

func MarshalReviewSession(session ReviewSession) ([]byte, error) {
	if err := session.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(session)
}

func DecodeReviewSession(raw []byte) (ReviewSession, error) {
	var session ReviewSession
	if len(raw) == 0 || len(raw) > 64<<10 {
		return session, errors.New("review session is empty or oversized")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&session); err != nil {
		return ReviewSession{}, errors.New("review session JSON is invalid or contains unknown fields")
	}
	if decoder.Decode(new(any)) != io.EOF {
		return ReviewSession{}, errors.New("review session must contain one JSON value")
	}
	if err := session.Validate(); err != nil {
		return ReviewSession{}, err
	}
	return session, nil
}

func validSessionDigest(value evidence.Digest) bool {
	text := string(value)
	if len(text) != 71 || !strings.HasPrefix(text, "sha256:") || text != strings.ToLower(text) {
		return false
	}
	_, err := hex.DecodeString(text[7:])
	return err == nil
}
