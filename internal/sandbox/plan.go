// Package sandbox runs explicitly approved, frozen inputs in offline Docker
// isolation. Docker and its host kernel remain trusted; this is not a VM boundary.
package sandbox

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

// Image is provisioned separately, never pulled by Execute.
const Image = "docker.io/library/golang@sha256:e0174e51e81218523251d85d248a90d24c3d5e81543b4f07a5d66229397db190"
const maxInput = 8 << 20
const maxGenerated = (8 << 20) + (64 << 10)

var ErrConsent = errors.New("sandbox plan was not approved or changed")
var ErrOutput = errors.New("sandbox output limit exceeded")
var ErrCommandStatus = errors.New("command status is reserved for helper/build/exec failure or possible signal")

var imagePattern = regexp.MustCompile(`^[a-zA-Z0-9._/:+-]+@sha256:[a-f0-9]{64}$`)
var platformPattern = regexp.MustCompile(`^linux/(amd64|arm64)$`)

// Limits are deliberately capped; time includes preflight, copy, build and run.
type Limits struct {
	Seconds     int `json:"seconds"`
	OutputBytes int `json:"output_bytes"`
}

type GeneratedFile struct {
	Path     string `json:"path"`
	Producer string `json:"producer"`
	MaxBytes int    `json:"max_bytes"`
	Mode     int64  `json:"mode"`
	Digest   string `json:"digest,omitempty"`
}

// Plan contains private copies; neither Preview nor caller mutation can change it.
type Plan struct {
	spec    specification
	archive []byte
	files   map[string][]byte
	slots   map[string]GeneratedFile
	stdin   []byte
}

type specification struct {
	Version        int             `json:"version"`
	Preparation    string          `json:"preparation"`
	Mounts         string          `json:"mounts"`
	Image          string          `json:"image"`
	Platform       string          `json:"platform"`
	Snapshot       string          `json:"snapshot"`
	Input          string          `json:"input_archive"`
	StdinDigest    string          `json:"stdin_digest,omitempty"`
	GeneratedSlots []GeneratedFile `json:"generated_slots,omitempty"`
	Materialized   []GeneratedFile `json:"materialized_files,omitempty"`
	Argv           []string        `json:"argv"`
	Environment    []string        `json:"environment"`
	Policy         []string        `json:"docker_policy"`
	Limits         Limits          `json:"limits"`
}

var environment = []string{"PATH=/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin", "HOME=/work", "TMPDIR=/work", "GOCACHE=/work/cache", "GOPATH=/work/go", "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "CGO_ENABLED=0", "GOMAXPROCS=2", "LANG=C"}
var policy = []string{"--network=none", "--read-only", "--user=65534:65534", "--cap-drop=ALL", "--security-opt=no-new-privileges", "--cgroupns=private", "--ipc=private", "--init", "--cpus=1", "--memory=1g", "--memory-swap=1g", "--pids-limit=128", "--ulimit=nofile=256:256", "--ulimit=core=0:0", "--tmpfs=/work:rw,exec,nosuid,nodev,size=536870912,mode=1777", "--shm-size=1m", "--log-driver=none", "--restart=no", "--workdir=/input", "--entrypoint=/usr/bin/env", "--pull=never"}

// Prepare accepts regular files only, not filesystem paths. The caller must load
// an already captured snapshot, not let Docker bind-mount a mutable checkout.
func Prepare(snapshot string, files map[string][]byte, argv []string, limits Limits) (*Plan, error) {
	return PrepareImage(snapshot, files, argv, limits, Image, "linux/"+runtime.GOARCH)
}

// PrepareImage creates a pull-free plan for one explicit digest-pinned image
// and platform. It does not inspect Docker or execute the image.
func PrepareImage(snapshot string, files map[string][]byte, argv []string, limits Limits, image, platform string) (*Plan, error) {
	return prepareImage(snapshot, files, argv, limits, image, platform, nil)
}

// PrepareCommandImage freezes a bounded stdin byte stream into an otherwise
// ordinary offline plan. Only the approved container receives those bytes.
func PrepareCommandImage(snapshot string, files map[string][]byte, argv []string, limits Limits, image, platform string, stdin []byte) (*Plan, error) {
	if len(stdin) > 16<<10 {
		return nil, errors.New("command stdin exceeds its byte limit")
	}
	plan, err := prepareImage(snapshot, files, argv, limits, image, platform, nil)
	if err != nil {
		return nil, err
	}
	plan.stdin = append([]byte(nil), stdin...)
	plan.spec.StdinDigest = digest(stdin)
	plan.spec.Preparation += "; attach frozen bounded stdin to the approved container"
	return plan, nil
}

// PrepareTemplate additionally reserves fixed trusted-runtime files whose bytes
// will be supplied only after the approved preparation recipe succeeds.
func PrepareTemplate(snapshot string, files map[string][]byte, argv []string, limits Limits, image, platform string, slots []GeneratedFile) (*Plan, error) {
	if len(slots) == 0 {
		return nil, errors.New("generated-file slot required")
	}
	copySlots := append([]GeneratedFile(nil), slots...)
	return prepareImage(snapshot, files, argv, limits, image, platform, copySlots)
}

func prepareImage(snapshot string, files map[string][]byte, argv []string, limits Limits, image, platform string, slots []GeneratedFile) (*Plan, error) {
	if !validDigest(snapshot) || len(files) == 0 || len(files) > 256 || len(argv) == 0 || len(argv) > 64 || !strings.HasPrefix(argv[0], "/") || limits.Seconds < 1 || limits.Seconds > 300 || limits.OutputBytes < 1 || limits.OutputBytes > 1<<20 || !imagePattern.MatchString(image) || !platformPattern.MatchString(platform) {
		return nil, errors.New("invalid sandbox plan bounds")
	}
	for _, arg := range argv {
		if len(arg) > 4096 || strings.ContainsRune(arg, 0) {
			return nil, errors.New("invalid argv")
		}
	}
	ownedFiles := make(map[string][]byte, len(files))
	for name, data := range files {
		ownedFiles[name] = append([]byte(nil), data...)
	}
	archive, err := makeArchive(ownedFiles, maxInput)
	if err != nil {
		return nil, err
	}
	slotMap := make(map[string]GeneratedFile, len(slots))
	var generatedBudget int
	for _, slot := range slots {
		if !validInputPath(slot.Path) || slot.Path == "" || slot.Producer == "" || len(slot.Producer) > 256 || slot.MaxBytes < 1 || slot.MaxBytes > maxGenerated || slot.Mode != 0444 && slot.Mode != 0555 {
			return nil, errors.New("invalid generated-file slot")
		}
		if _, exists := ownedFiles[slot.Path]; exists {
			return nil, errors.New("generated-file slot collides with captured input")
		}
		if _, exists := slotMap[slot.Path]; exists {
			return nil, errors.New("duplicate generated-file slot")
		}
		if (slot.Path == "after/launcher" && (slot.Mode != 0555 || slot.MaxBytes > 8<<20)) || (slot.Path == "after/service.json" && (slot.Mode != 0444 || slot.MaxBytes > 64<<10)) || (slot.Path != "after/launcher" && slot.Path != "after/service.json") {
			return nil, errors.New("generated files are limited to the fixed trusted-runtime overlay")
		}
		generatedBudget += slot.MaxBytes
		slotMap[slot.Path] = slot
	}
	if generatedBudget > maxGenerated {
		return nil, errors.New("generated-file transfer budget exceeded")
	}
	return &Plan{spec: specification{Version: 1, Preparation: "copy bounded regular files to /input in an unstarted container; commit input-only image; run read-only; remove both containers and derived image", Mounts: "no host mounts, volumes, sockets or published ports; only bounded /work and /dev/shm tmpfs", Image: image, Platform: platform, Snapshot: snapshot, Input: digest(archive), GeneratedSlots: append([]GeneratedFile(nil), slots...), Argv: append([]string(nil), argv...), Environment: append([]string(nil), environment...), Policy: append([]string(nil), policy...), Limits: limits}, archive: archive, files: ownedFiles, slots: slotMap}, nil
}

// Materialize fills only generated slots declared in the approved template.
// Generated bytes are independently bounded and their digest is retained in
// the resulting concrete plan; all captured source remains unchanged.
// WithCommandInput returns a private plan copy whose preview binds the exact
// bounded stdin bytes that ExecuteCommand later attaches to the container.
func (p *Plan) WithCommandInput(stdin []byte) (*Plan, error) {
	if p == nil || len(stdin) > 16<<10 {
		return nil, errors.New("command stdin exceeds its byte limit")
	}
	copyPlan := *p
	copyPlan.stdin = append([]byte(nil), stdin...)
	copyPlan.spec.StdinDigest = digest(stdin)
	copyPlan.spec.Preparation += "; attach frozen bounded stdin to the approved container"
	return &copyPlan, nil
}

func (p *Plan) Materialize(files map[string][]byte) (*Plan, error) {
	if p == nil || len(p.slots) == 0 || len(files) != len(p.slots) {
		return nil, errors.New("generated files do not match approved template")
	}
	merged := make(map[string][]byte, len(p.files)+len(files))
	for name, data := range p.files {
		merged[name] = append([]byte(nil), data...)
	}
	materialized := make([]GeneratedFile, 0, len(files))
	var total int
	for name, data := range files {
		slot, ok := p.slots[name]
		if !ok || len(data) > slot.MaxBytes {
			return nil, errors.New("generated file is missing, undeclared, or over budget")
		}
		total += len(data)
		if total > maxGenerated {
			return nil, errors.New("generated-file transfer budget exceeded")
		}
		merged[name] = append([]byte(nil), data...)
		slot.Digest = digest(data)
		materialized = append(materialized, slot)
	}
	sort.Slice(materialized, func(i, j int) bool { return materialized[i].Path < materialized[j].Path })
	archive, err := makeArchive(merged, maxInput+maxGenerated)
	if err != nil {
		return nil, err
	}
	concrete := *p
	concrete.archive = archive
	concrete.files = merged
	concrete.slots = nil
	concrete.spec.Input = digest(archive)
	concrete.spec.GeneratedSlots = nil
	concrete.spec.Materialized = materialized
	concrete.spec.Preparation += "; materialize approved generated runtime files using fixed bounded slots"
	return &concrete, nil
}

func makeArchive(files map[string][]byte, maxBytes int) ([]byte, error) {
	if len(files) == 0 || len(files) > 258 {
		return nil, errors.New("invalid input archive file count")
	}
	names := make([]string, 0, len(files))
	total := 0
	for name, data := range files {
		if !validInputPath(name) {
			return nil, errors.New("invalid input path")
		}
		total += len(data)
		if total > maxBytes {
			return nil, errors.New("input budget exceeded")
		}
		names = append(names, name)
	}
	sort.Strings(names)
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	dirs := map[string]bool{}
	for _, name := range names {
		parent := path.Dir("input/" + name)
		var parents []string
		for parent != "." {
			parents = append(parents, parent)
			parent = path.Dir(parent)
		}
		for i := len(parents) - 1; i >= 0; i-- {
			d := parents[i]
			if !dirs[d] {
				if err := tw.WriteHeader(&tar.Header{Name: d + "/", Mode: 0555, Typeflag: tar.TypeDir}); err != nil {
					return nil, err
				}
				dirs[d] = true
			}
		}
		mode := int64(0444)
		if generated, ok := generatedFileMode(name, files); ok {
			mode = generated
		}
		if err := tw.WriteHeader(&tar.Header{Name: "input/" + name, Mode: mode, Size: int64(len(files[name])), Typeflag: tar.TypeReg}); err != nil {
			return nil, err
		}
		if _, err := tw.Write(files[name]); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Generated launcher path is fixed by the HTTP-service contract. Runtime
// configuration is data and remains non-executable.
func generatedFileMode(name string, _ map[string][]byte) (int64, bool) {
	if name == "after/launcher" {
		return 0555, true
	}
	if name == "after/service.json" {
		return 0444, true
	}
	return 0, false
}

func validInputPath(name string) bool {
	return name != "" && name != "." && name != ".." && path.Clean(name) == name && !strings.HasPrefix(name, "/") && !strings.HasPrefix(name, "../") && !strings.ContainsAny(name, "\\\x00\r\n") && len(name) <= 240
}

// Preview is plain JSON. UI callers must still escape terminal controls in argv.
func (p *Plan) Preview() ([]byte, string) {
	b, _ := json.MarshalIndent(p.spec, "", "  ")
	return b, digest(b)
}
func digest(b []byte) string { sum := sha256.Sum256(b); return fmt.Sprintf("sha256:%x", sum) }
func validDigest(s string) bool {
	if len(s) != 71 || !strings.HasPrefix(s, "sha256:") {
		return false
	}
	_, err := hex.DecodeString(s[7:])
	return err == nil && strings.ToLower(s) == s
}
func (p *Plan) timeout() time.Duration { return time.Duration(p.spec.Limits.Seconds) * time.Second }
