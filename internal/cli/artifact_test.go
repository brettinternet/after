package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/brettinternet/after/internal/store"
)

func TestInspectArtifactPages(t *testing.T) {
	project := t.TempDir()
	makeProject(t, project)
	config := filepath.Join(project, "empty.yaml")
	if err := os.WriteFile(config, nil, 0600); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(project, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, raw := range [][]byte{
		[]byte(`{"seconds":43200,"responses":[],"calls":[]}`),
		[]byte("untrusted\x1b]52;c;bad\a\x00"),
		[]byte(`{"kind":"observed","amount":9007199254740993}`),
		bytes.Repeat([]byte("x"), 65537),
	} {
		artifact, err := s.PutArtifact(raw, "test", store.MaxBlobBytes)
		if err != nil {
			t.Fatal(err)
		}
		for _, command := range []string{"inspect", "export"} {
			var recovered []byte
			for offset := 0; ; {
				code, output, stderr := invoke([]string{command, string(artifact.Content), "--project", project, "--config", config, "--artifact-offset", strconv.Itoa(offset)}, false, "")
				if code != ExitOK || stderr != "" {
					t.Fatalf("%d %s %s", code, output, stderr)
				}
				if strings.ContainsAny(output, "\x1b\a\x00") {
					t.Fatal("raw terminal control leaked")
				}
				var page struct {
					Kind string `json:"kind"`
					Data struct {
						Next     int             `json:"next"`
						More     bool            `json:"more"`
						Base64   string          `json:"base64"`
						Document json.RawMessage `json:"document"`
					} `json:"data"`
				}
				if err := json.Unmarshal([]byte(output), &page); err != nil {
					t.Fatal(err)
				}
				if page.Kind != "artifact" {
					t.Fatalf("content was promoted to evidence: %s", output)
				}
				decoded, err := base64.StdEncoding.DecodeString(page.Data.Base64)
				if err != nil || len(decoded) > 65536 {
					t.Fatal("invalid bounded page")
				}
				recovered = append(recovered, decoded...)
				if len(raw) <= 65536 && json.Valid(raw) {
					if !bytes.Equal(page.Data.Document, raw) {
						t.Fatalf("document precision changed: %s", output)
					}
				} else if len(page.Data.Document) != 0 {
					t.Fatal("partial/binary document must not be JSON")
				}
				if !page.Data.More {
					break
				}
				if page.Data.Next <= offset {
					t.Fatal("page did not advance")
				}
				offset = page.Data.Next
			}
			if !bytes.Equal(recovered, raw) {
				t.Fatal("artifact bytes changed")
			}
		}
		for _, flags := range [][]string{{"--artifact-offset", "-1"}, {"--artifact-size", "0"}, {"--artifact-size", "65537"}} {
			args := append([]string{"inspect", string(artifact.Content), "--project", project, "--config", config}, flags...)
			code, output, _ := invoke(args, false, "")
			if code != ExitInvalid || output != "" {
				t.Fatal("invalid artifact bounds accepted")
			}
		}
	}
}
