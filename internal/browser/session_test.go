package browser

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/brettinternet/after/internal/capture"
	"github.com/brettinternet/after/internal/evidence"
)

func TestReviewSessionRoundTripAndCaptureFlags(t *testing.T) {
	pair := evidence.SnapshotPair{Base: sessionDigest("a"), Candidate: sessionDigest("b")}
	options := capture.Options{Mode: evidence.MergeBase, Base: "main", Target: "HEAD", IncludeUntracked: []string{"fixtures/input.json"}}
	session := NewReviewSession(pair, options)
	raw, err := MarshalReviewSession(session)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeReviewSession(raw)
	if err != nil || !reflect.DeepEqual(decoded, session) {
		t.Fatalf("session round trip: %+v %v", decoded, err)
	}
	if got := decoded.CaptureOptions(); got.Mode != options.Mode || got.Base != options.Base || got.Target != options.Target || len(got.IncludeUntracked) != 1 || got.IncludeUntracked[0] != options.IncludeUntracked[0] {
		t.Fatalf("capture flags changed: %+v", got)
	}
	if strings.Contains(string(raw), "source") || strings.Contains(string(raw), "fixtures/input.json\n") {
		t.Fatalf("session contains source content: %s", raw)
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil || len(object) != 4 {
		t.Fatalf("unexpected session fields: %s %v", raw, err)
	}
}

func TestReviewSessionRejectsUntrustedShapes(t *testing.T) {
	valid := ReviewSession{SchemaVersion: ReviewSessionVersion, Pair: evidence.SnapshotPair{Base: sessionDigest("a"), Candidate: sessionDigest("b")}, Mode: evidence.OriginalBase}
	raw, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	mutations := map[string]string{
		"unknown field":         strings.TrimSuffix(string(raw), "}") + `,"source":"payload"}`,
		"invalid base ID":       strings.Replace(string(raw), string(valid.Pair.Base), "sha256:bad", 1),
		"invalid candidate ID":  strings.Replace(string(raw), string(valid.Pair.Candidate), "sha256:bad", 1),
		"unsupported mode":      strings.Replace(string(raw), "original_base", "last_inspected", 1),
		"staged and merge-base": strings.TrimSuffix(string(raw), "}") + `,"capture":{"staged":true,"base":"main","target":"HEAD"}}`,
		"trailing value":        string(raw) + `{}`,
	}
	for name, input := range mutations {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeReviewSession([]byte(input)); err == nil {
				t.Fatal("accepted invalid session")
			}
		})
	}
	if _, err := DecodeReviewSession(make([]byte, (64<<10)+1)); err == nil {
		t.Fatal("accepted oversized session")
	}
}

func sessionDigest(char string) evidence.Digest {
	return evidence.Digest("sha256:" + strings.Repeat(char, 64))
}
