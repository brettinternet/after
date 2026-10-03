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
	"sort"
	"strings"
	"time"
)

// Image is provisioned separately, never pulled by Execute.
const Image = "docker.io/library/golang@sha256:e0174e51e81218523251d85d248a90d24c3d5e81543b4f07a5d66229397db190"
const maxInput = 8 << 20

var ErrConsent = errors.New("sandbox plan was not approved or changed")
var ErrOutput = errors.New("sandbox output limit exceeded")

// Limits are deliberately capped; time includes preflight, copy, build and run.
type Limits struct {
	Seconds     int `json:"seconds"`
	OutputBytes int `json:"output_bytes"`
}

// Plan contains private copies; neither Preview nor caller mutation can change it.
type Plan struct {
	spec    specification
	archive []byte
}

type specification struct {
	Version     int      `json:"version"`
	Preparation string   `json:"preparation"`
	Mounts      string   `json:"mounts"`
	Image       string   `json:"image"`
	Snapshot    string   `json:"snapshot"`
	Input       string   `json:"input_archive"`
	Argv        []string `json:"argv"`
	Environment []string `json:"environment"`
	Policy      []string `json:"docker_policy"`
	Limits      Limits   `json:"limits"`
}

var environment = []string{"PATH=/usr/local/go/bin:/usr/bin:/bin", "HOME=/work", "TMPDIR=/work", "GOCACHE=/work/cache", "GOPATH=/work/go", "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "CGO_ENABLED=0", "GOMAXPROCS=2", "LANG=C"}
var policy = []string{"--network=none", "--read-only", "--user=65534:65534", "--cap-drop=ALL", "--security-opt=no-new-privileges", "--cgroupns=private", "--ipc=private", "--init", "--cpus=1", "--memory=1g", "--memory-swap=1g", "--pids-limit=128", "--ulimit=nofile=256:256", "--ulimit=core=0:0", "--tmpfs=/work:rw,exec,nosuid,nodev,size=536870912,mode=1777", "--shm-size=1m", "--log-driver=none", "--restart=no", "--workdir=/input", "--entrypoint=/usr/bin/env", "--pull=never"}

// Prepare accepts regular files only, not filesystem paths. The caller must load
// an already captured snapshot, not let Docker bind-mount a mutable checkout.
func Prepare(snapshot string, files map[string][]byte, argv []string, limits Limits) (*Plan, error) {
	if !validDigest(snapshot) || len(files) == 0 || len(files) > 256 || len(argv) == 0 || len(argv) > 64 || !strings.HasPrefix(argv[0], "/") || limits.Seconds < 1 || limits.Seconds > 300 || limits.OutputBytes < 1 || limits.OutputBytes > 1<<20 {
		return nil, errors.New("invalid sandbox plan bounds")
	}
	for _, arg := range argv {
		if len(arg) > 4096 || strings.ContainsRune(arg, 0) {
			return nil, errors.New("invalid argv")
		}
	}
	names := make([]string, 0, len(files))
	total := 0
	for name, data := range files {
		if name == "." || name == ".." || path.Clean(name) != name || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "../") || strings.ContainsAny(name, "\\\x00\r\n") || len(name) > 240 {
			return nil, errors.New("invalid input path")
		}
		total += len(data)
		if total > maxInput {
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
		if err := tw.WriteHeader(&tar.Header{Name: "input/" + name, Mode: 0444, Size: int64(len(files[name])), Typeflag: tar.TypeReg}); err != nil {
			return nil, err
		}
		if _, err := tw.Write(files[name]); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return &Plan{spec: specification{Version: 1, Preparation: "copy bounded regular files to /input in an unstarted container; commit input-only image; run read-only; remove both containers and derived image", Mounts: "no host mounts, volumes, sockets or published ports; only bounded /work and /dev/shm tmpfs", Image: Image, Snapshot: snapshot, Input: digest(buf.Bytes()), Argv: append([]string(nil), argv...), Environment: append([]string(nil), environment...), Policy: append([]string(nil), policy...), Limits: limits}, archive: buf.Bytes()}, nil
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
