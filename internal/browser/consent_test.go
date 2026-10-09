package browser

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/runner"
	"github.com/brettinternet/after/internal/sandbox"
	"github.com/brettinternet/after/internal/terminal"
	tea "github.com/charmbracelet/bubbletea"
)

func consentTestDigest(char byte) evidence.Digest {
	return evidence.Digest("sha256:" + strings.Repeat(string(char), 64))
}

func consentTestPlan() consentPreview {
	pair := evidence.SnapshotPair{Base: consentTestDigest('a'), Candidate: consentTestDigest('b')}
	definition := runner.BuiltinPaymentDefinition(1, sandbox.Limits{Seconds: 180, OutputBytes: 65536})
	// Keep synthetic plans and goldens independent of the host architecture.
	definition.Platform = "linux/arm64"
	definition.Environment = []string{}
	scenario := evidence.Scenario{
		SchemaVersion: evidence.SchemaVersion,
		ID:            consentTestDigest('c'),
		Input:         consentTestDigest('d'),
		Driver:        consentTestDigest('e'),
		Observer:      consentTestDigest('f'),
		Rules:         consentTestDigest('1'),
		Boundary:      "synthetic fixture only",
		Author:        "AFTER test runner",
		Limits:        []string{"finite test scope"},
	}
	scenario.Input = consentTestDigest('d')
	definitionDigest := scenario.Input
	limits := sandbox.Limits{Seconds: definition.Limits.Seconds, OutputBytes: definition.Limits.OutputBytes}
	makeSandboxPlan := func(snapshot, image, platform string, argv []string, planLimits sandbox.Limits, slots []sandbox.GeneratedFile) consentSandboxPlan {
		return consentSandboxPlan{
			Version:        1,
			Preparation:    "copy bounded regular files to /input in an unstarted container; commit input-only image; run read-only",
			Mounts:         "no host mounts, volumes, sockets or published ports; only bounded /work and /dev/shm tmpfs",
			Image:          image,
			Platform:       platform,
			Snapshot:       snapshot,
			Input:          string(consentTestDigest('2')),
			GeneratedSlots: slots,
			Argv:           argv,
			Environment:    []string{"PATH=/usr/local/go/bin:/usr/bin:/bin", "GOPROXY=off", "GOSUMDB=off"},
			Policy:         []string{"--network=none", "--read-only", "--user=65534:65534", "--cap-drop=ALL"},
			Limits:         planLimits,
		}
	}
	preparation := makeSandboxPlan(string(definitionDigest), sandbox.Image, definition.Platform, []string{"/usr/local/go/bin/go", "run", "/input/preparer.go"}, sandbox.Limits{Seconds: definition.Limits.PreparationSeconds, OutputBytes: definition.Limits.OutputBytes}, nil)
	experiments := [][]consentExperiment{make([]consentExperiment, len(definition.Cases)), make([]consentExperiment, len(definition.Cases))}
	for side := range experiments {
		snapshot := string(pair.Base)
		if side == 1 {
			snapshot = string(pair.Candidate)
		}
		for caseIndex, scenarioCase := range definition.Cases {
			slots := []sandbox.GeneratedFile{
				{Path: "after/launcher", Producer: string(consentTestDigest('8')), MaxBytes: 8 << 20, Mode: 0555},
				{Path: "after/service.json", Producer: string(definitionDigest), MaxBytes: 64 << 10, Mode: 0444},
			}
			app := makeSandboxPlan(snapshot, definition.Image, definition.Platform, []string{"/input/after/launcher"}, limits, slots)
			observer := makeSandboxPlan(string(scenario.Driver), sandbox.Image, definition.Platform, []string{"/usr/local/go/bin/go", "run", "/input/observer.go", "/input/definition.json", scenarioCase.ID}, limits, nil)
			experiments[side][caseIndex] = consentExperiment{
				App:      app,
				Observer: observer,
				Topology: "app network=none; observer joins app network only; separate PID, IPC, filesystem and tmpfs; no host ports; app terminated after observation",
			}
		}
	}
	return consentPreview{
		Version:           1,
		Request:           consentTestDigest('3'),
		Snapshots:         pair,
		Scenario:          scenario,
		DefinitionDigest:  definitionDigest,
		Definition:        definition,
		DefinitionSource:  runner.DefinitionSource{Kind: "built-in-payment"},
		ServiceConfig:     json.RawMessage(`{"version":1,"start_argv":["/work/app"]}`),
		Repetitions:       definition.Repetitions,
		Limits:            limits,
		Preparation:       preparation,
		PreparationBudget: fmt.Sprintf("one in-approved-request launcher build; %d seconds; stdout ≤8 MiB binary, stderr ≤%d bytes diagnostics; no cache", definition.Limits.PreparationSeconds, definition.Limits.OutputBytes),
		Concurrency:       1,
		Experiments:       experiments,
	}
}

func consentTestBytes(t *testing.T, preview consentPreview) []byte {
	t.Helper()
	raw, err := json.MarshalIndent(preview, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func commandConsentTestDefinition() runner.CommandDefinition {
	return runner.CommandDefinition{
		Version: 1, Kind: "command", Name: "synthetic-command", Platform: "linux/arm64", Image: sandbox.Image,
		BuildArgv: []string{"/usr/local/go/bin/go", "build", "-o", "/work/fixture", "/input/main.go"},
		Cases: []runner.CommandCase{
			{ID: "changed", Title: "Source-dependent output", Argv: []string{"/work/fixture", "changed"}, Stdin: []byte("request=changed\n"), Environment: []string{"FIXTURE_MODE=changed"}, InputFiles: []runner.CommandInputFile{{Path: "fixtures/changed.json", Content: []byte(`{"case":"changed"}`)}}},
			{ID: "control", Title: "Unaffected control", Argv: []string{"/work/fixture", "control"}, Stdin: []byte("request=control\n"), Environment: []string{"FIXTURE_MODE=control", "FEATURE=off"}, InputFiles: []runner.CommandInputFile{{Path: "fixtures/control.json", Content: []byte(`{"case":"control"}`)}}},
		},
		Repetitions: 1, Limits: runner.DefinitionLimits{Seconds: 60, OutputBytes: 65536, PreparationSeconds: 90},
		Comparison: runner.CommandComparison{Stdout: "text", Stderr: "json"},
	}
}

func commandConsentTestPlan() commandConsentPreview {
	return commandConsentTestPlanFor(commandConsentTestDefinition())
}

func commandConsentTestPlanFor(definition runner.CommandDefinition) commandConsentPreview {
	rawDefinition, err := json.Marshal(definition)
	if err != nil {
		panic(err)
	}
	definitionDigest := evidence.Digest(consentTestDigestOf(rawDefinition))
	pair := evidence.SnapshotPair{Base: consentTestDigest('a'), Candidate: consentTestDigest('b')}
	limits := sandbox.Limits{Seconds: definition.Limits.Seconds, OutputBytes: definition.Limits.OutputBytes}
	preparationLimits := sandbox.Limits{Seconds: definition.Limits.PreparationSeconds, OutputBytes: definition.Limits.OutputBytes}
	preparationPlan, err := sandbox.PrepareImage(string(definitionDigest), map[string][]byte{"preparer.go": []byte("trusted preparation source")}, []string{"/usr/local/go/bin/go", "run", "/input/preparer.go"}, preparationLimits, sandbox.Image, definition.Platform)
	if err != nil {
		panic(err)
	}
	preparationRaw, preparationID := preparationPlan.Preview()
	var preparation consentSandboxPlan
	if err := json.Unmarshal(preparationRaw, &preparation); err != nil {
		panic(err)
	}
	configs := make([]json.RawMessage, len(definition.Cases))
	for index, scenarioCase := range definition.Cases {
		config, err := json.Marshal(commandLauncherConfig{Version: 1, BuildArgv: definition.BuildArgv, Argv: scenarioCase.Argv, Environment: scenarioCase.Environment})
		if err != nil {
			panic(err)
		}
		configs[index] = config
	}
	experiments := [][]commandConsentExperiment{make([]commandConsentExperiment, len(definition.Cases)), make([]commandConsentExperiment, len(definition.Cases))}
	for side := range experiments {
		snapshot := pair.Base
		if side == 1 {
			snapshot = pair.Candidate
		}
		for caseIndex, scenarioCase := range definition.Cases {
			files := map[string][]byte{"main.go": []byte("synthetic command source")}
			for _, input := range scenarioCase.InputFiles {
				files[input.Path] = input.Content
			}
			template, err := sandbox.PrepareTemplate(string(snapshot), files, []string{"/input/after/launcher"}, limits, definition.Image, definition.Platform, []sandbox.GeneratedFile{
				{Path: "after/launcher", Producer: preparationID, MaxBytes: 8 << 20, Mode: 0555},
				{Path: "after/service.json", Producer: string(definitionDigest), MaxBytes: 64 << 10, Mode: 0444},
			})
			if err != nil {
				panic(err)
			}
			commandPlan, err := template.WithCommandInput(scenarioCase.Stdin)
			if err != nil {
				panic(err)
			}
			appRaw, _ := commandPlan.Preview()
			var app consentSandboxPlan
			if err := json.Unmarshal(appRaw, &app); err != nil {
				panic(err)
			}
			experiments[side][caseIndex] = commandConsentExperiment{App: app, Topology: commandTopology}
		}
	}
	scenario := evidence.Scenario{
		SchemaVersion: evidence.SchemaVersion, ID: consentTestDigest('c'), Input: definitionDigest,
		Driver: consentTestDigest('d'), Observer: consentTestDigest('e'), Rules: consentTestDigest('f'),
		Boundary: "Docker-inspected command status and attached streams", Author: "AFTER operator-selected command v1",
		Limits: []string{"finite command cases"},
	}
	return commandConsentPreview{
		Version: 1, Request: consentTestDigest('3'), Snapshots: pair, Scenario: scenario,
		DefinitionDigest: definitionDigest, Definition: definition,
		DefinitionSource: runner.DefinitionSource{Kind: "operator-selected-file", RepositoryPath: "scenarios/command.json"},
		CommandConfigs:   configs, Repetitions: definition.Repetitions, Limits: limits,
		Preparation:       preparation,
		PreparationBudget: fmt.Sprintf("one in-approved-request launcher build; %d seconds; stdout ≤8 MiB binary, stderr ≤%d bytes diagnostics; no cache", definition.Limits.PreparationSeconds, definition.Limits.OutputBytes),
		Concurrency:       1, Experiments: experiments,
	}
}

func commandConsentTestBytes(t *testing.T, preview commandConsentPreview) []byte {
	t.Helper()
	raw, err := json.MarshalIndent(preview, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func commandConsentView(t *testing.T, raw []byte) *Model {
	t.Helper()
	var preview commandConsentPreview
	if err := json.Unmarshal(raw, &preview); err != nil {
		t.Fatal(err)
	}
	summary, summaryErr := ConsentSummary(raw)
	m := New(t.Context(), Selection{Project: "payment", Pair: preview.Snapshots}, Jobs{})
	t.Cleanup(m.Close)
	m.theme.Color = false
	_, cmd := m.Update(prepared{pair: preview.Snapshots, raw: raw, digest: consentTestDigestOf(raw), summary: summary, summaryErr: summaryErr})
	drain(m, cmd)
	return m
}

func consentTestDigestOf(raw []byte) string {
	sum := sha256.Sum256(raw)
	return fmt.Sprintf("sha256:%x", sum)
}

func consentView(t *testing.T, raw []byte) *Model {
	t.Helper()
	preview, err := decodeConsentPreview(raw)
	if err != nil {
		t.Fatal(err)
	}
	summary, summaryErr := ConsentSummary(raw)
	m := New(t.Context(), Selection{Project: "payment", Pair: preview.Snapshots}, Jobs{})
	t.Cleanup(m.Close)
	m.theme.Color = false
	_, cmd := m.Update(prepared{pair: preview.Snapshots, raw: raw, digest: consentTestDigestOf(raw), summary: summary, summaryErr: summaryErr})
	drain(m, cmd)
	return m
}

func TestConsentSummaryStrictDecodeDistinctValuesAndEscaping(t *testing.T) {
	preview := consentTestPlan()
	preview.DefinitionSource = runner.DefinitionSource{Kind: "operator-selected-file", RepositoryPath: "\nDocker policy: forged\x1b]52;c;clipboard\a", ChangedOracle: true}
	preview.Experiments[0][0].App.Environment = []string{"GOMAXPROCS=2\r\n    Limits: forged", "\x1b]52;c;clipboard\a"}
	preview.Experiments[1][1].Topology = "candidate topology\n    Image: forged"
	preview.Experiments[1][0].App.Image = "candidate image\x1b[2J"
	preview.Experiments[1][1].Observer.Mounts = "candidate mounts"
	preview.Experiments[0][1].App.Policy = []string{"--network=none", "--read-only", "--cap-drop=ALL"}
	raw := consentTestBytes(t, preview)
	summary, err := ConsentSummary(raw)
	if err != nil {
		t.Fatal(err)
	}
	text := string(summary)
	for _, required := range []string{
		"Snapshots:", "Runs: 2 sides × 2 cases × 1 repetition = 4 runs · concurrency 1",
		"Definition:", "Service image/platform:", "Start argv:", "Service environment:", "Readiness:", "Fake upstreams:", "Controlled-clock requests:", "Compared channels:",
		"Preparation recipe:", "Preparation budget and transfer:", "App and observer images:", "Container argv:", "Container environment:", "Input archives:", "Mounts:", "Docker policy:", "Container limits:", "Changed oracle:",
		`GOMAXPROCS=2\r\n    Limits: forged`, `\u001b]52;c;clipboard\u0007`,
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("summary missing %q:\n%s", required, text)
		}
	}
	if got := strings.Count(text, `"candidate topology\n    Image: forged"`); got != 1 {
		t.Fatalf("distinct topology value appeared %d times:\n%s", got, text)
	}
	if got := strings.Count(text, `"candidate image\u001b[2J / linux/arm64"`); got != 1 {
		t.Fatalf("distinct image value appeared %d times:\n%s", got, text)
	}
	if !strings.Contains(text, `"`+preview.Experiments[0][0].App.Image+` / `+preview.Experiments[0][0].App.Platform+`"`) || !strings.Contains(text, `"candidate image\u001b[2J / `+preview.Experiments[1][0].App.Platform+`"`) {
		t.Fatalf("disagreeing images are not listed distinctly:\n%s", text)
	}

	m := consentView(t, raw)
	press(m, "right")
	press(m, "right")
	frame := m.View()
	for _, bad := range []string{"\x1b]52;", "\x1b[2J", "\a", "\r"} {
		if strings.Contains(frame, bad) {
			t.Fatalf("hostile preview value escaped into terminal: %q", frame)
		}
	}
	if !strings.Contains(frame, `Docker policy: forged`) || !strings.Contains(frame, `\n`) {
		t.Fatalf("escaped hostile value was not visible as data: %q", frame)
	}
	for _, line := range strings.Split(frame, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "Docker policy: forged") || strings.HasPrefix(strings.TrimSpace(line), "Image: forged") {
			t.Fatalf("payload forged a summary label: %q", line)
		}
	}
}

func TestCommandConsentSummaryShowsBoundedCaseInputsAndExactInspection(t *testing.T) {
	definition := commandConsentTestDefinition()
	longPrefix := strings.Repeat("same-prefix-", 18)
	definition.Cases[0].Argv[1] = longPrefix + "changed"
	definition.Cases[1].Argv[1] = longPrefix + "control"
	definition.Cases[0].Title = "Hostile \x1b]52;c;clipboard\a title"
	definition.Cases[0].Environment[0] = "FIXTURE_MODE=changed\x1b]52;c;clipboard"
	definition.Cases[0].InputFiles[0].Path = "fixtures/\x1b[2J.json"
	definition.Cases[0].Stdin = []byte("\x1b]52;c;clipboard\a" + strings.Repeat("x", 20))
	preview := commandConsentTestPlanFor(definition)
	raw := commandConsentTestBytes(t, preview)
	summary, err := ConsentSummary(raw)
	if err != nil {
		t.Fatal(err)
	}
	text := string(summary)
	for _, required := range []string{
		"Command consent", "Case 1:", "Case 2:", `id: "changed"`, `id: "control"`, `title:`,
		"argv[0]", "argv[1]", "environment[0]", "environment[1]", "input[0]", "size=18 B", "stdin: size=37 B",
		"preview=", "Image: docker.io/library/golang", "Image SHA-256:", "linux/arm64", "Build argv (direct; no shell)",
		"Comparison: stdout=text · stderr=json · exit status=exact", "Definition limits:",
		"Docker policy (command containers)", `"--interactive"`, `"--read-only"`,
		"Plan ID (concatenate the next two lines without spaces):\n" + consentTestDigestOf(raw)[:commandConsentPlanIDPartWidth] + "\n" + consentTestDigestOf(raw)[commandConsentPlanIDPartWidth:], "after inspect PLAN", "after inspect PLAN --json",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("command summary missing %q:\n%s", required, text)
		}
	}
	for _, value := range []string{definition.Cases[0].Argv[1], definition.Cases[1].Argv[1], "\x1b]52;", "\a"} {
		if strings.Contains(text, value) {
			t.Fatalf("command summary exposed raw/long value %q", value)
		}
	}
	for _, arg := range definition.Cases[:2] {
		sum := sha256.Sum256([]byte(arg.Argv[1]))
		if !strings.Contains(text, fmt.Sprintf("…#%x", sum[:4])) {
			t.Fatalf("long argv value lacks its distinguishing fingerprint: %s", text)
		}
	}
	stdinSum := sha256.Sum256(definition.Cases[0].Stdin)
	if !strings.Contains(text, fmt.Sprintf("…#%x", stdinSum[:4])) || !strings.Contains(text, `\x1b`) {
		t.Fatalf("stdin preview was not escaped or fingerprinted: %s", text)
	}
	if len(summary) > 64<<10 {
		t.Fatalf("command summary exceeded its bounded projection: %d bytes", len(summary))
	}
	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		if terminal.Line(line, commandConsentLineWidth) != line {
			t.Fatalf("summary line exceeds %d display columns: %q", commandConsentLineWidth, line)
		}
	}
}

func TestConsentDecoderRejectsUnknownNestedFieldsAndTrailingData(t *testing.T) {
	raw := consentTestBytes(t, consentTestPlan())
	for name, bad := range map[string][]byte{
		"sandbox":       bytes.Replace(raw, []byte(`"App": {`), []byte(`"App": {"future_field": true,`), 1),
		"scenario":      bytes.Replace(raw, []byte(`"scenario": {`), []byte(`"scenario": {"future_field": true,`), 1),
		"trailing JSON": append(append([]byte(nil), raw...), []byte(` {}`)...),
		"invalid UTF-8": append(append([]byte(nil), raw...), 0xff),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeConsentPreview(bad); err == nil {
				t.Fatal("strict consent decoder accepted malformed preview")
			}
			if _, err := ConsentSummary(bad); err == nil {
				t.Fatal("malformed preview produced an approval summary")
			}
		})
	}
}

func TestConsentDecoderRejectsExperimentDimensions(t *testing.T) {
	for _, shape := range []struct {
		name         string
		sides, cases int
	}{
		{"extra side", 3, 2},
		{"missing side", 1, 2},
		{"extra case", 2, 3},
		{"missing case", 2, 1},
	} {
		t.Run(shape.name, func(t *testing.T) {
			preview := consentTestPlan()
			sample := preview.Experiments[0][0]
			preview.Experiments = make([][]consentExperiment, shape.sides)
			for side := range preview.Experiments {
				preview.Experiments[side] = make([]consentExperiment, shape.cases)
				for c := range preview.Experiments[side] {
					preview.Experiments[side][c] = sample
				}
			}
			raw := consentTestBytes(t, preview)
			if _, err := decodeConsentPreview(raw); err == nil {
				t.Fatal("accepted unsupported experiment dimensions")
			}
			if _, err := ConsentSummary(raw); err == nil {
				t.Fatal("unsupported dimensions produced a misleading summary")
			}
		})
	}
}

func consentGoldenViews(t *testing.T) {
	raw := consentTestBytes(t, consentTestPlan())
	for _, size := range []tea.WindowSizeMsg{{Width: 120, Height: 40}, {Width: 80, Height: 24}} {
		name := fmt.Sprintf("consent-%dx%d", size.Width, size.Height)
		t.Run(name, func(t *testing.T) {
			m := consentView(t, raw)
			step(m, size)
			got := m.View() + "\n"
			path := filepath.Join("testdata", "views", name+".txt")
			if *updateViews {
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(want) != got {
				t.Fatalf("consent view differs: %s; regenerate explicitly with -update\n%s", path, got)
			}
			for _, line := range strings.Split(got, "\n") {
				if terminalWidth := terminal.Line(line, size.Width); terminalWidth != line {
					t.Fatalf("golden row exceeds %d columns: %q", size.Width, line)
				}
			}
		})
	}
}

func commandConsentGoldenViews(t *testing.T) {
	raw := commandConsentTestBytes(t, commandConsentTestPlan())
	for _, size := range []tea.WindowSizeMsg{{Width: 120, Height: 40}, {Width: 80, Height: 24}} {
		name := fmt.Sprintf("command-consent-%dx%d", size.Width, size.Height)
		t.Run(name, func(t *testing.T) {
			m := commandConsentView(t, raw)
			step(m, size)
			got := m.View() + "\n"
			path := filepath.Join("testdata", "views", name+".txt")
			if *updateViews {
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(want) != got {
				t.Fatalf("command consent view differs: %s; regenerate explicitly with -update\n%s", path, got)
			}
			for _, line := range strings.Split(got, "\n") {
				if terminal.Line(line, size.Width) != line {
					t.Fatalf("golden row exceeds %d columns: %q", size.Width, line)
				}
			}
		})
	}
}

func TestConsentExactPlanViewerAndModalKeys(t *testing.T) {
	m, _, _ := newConsentActionModel(t)
	preview := append([]byte(nil), m.preview...)
	digest := m.digest
	for _, forbidden := range []string{"/", "a", "c", "d", "i", "p", "r", "s", "u", "x", "1", "2", "3"} {
		press(m, forbidden)
		if m.screen != "plan" || !bytes.Equal(m.preview, preview) || m.digest != digest || m.running {
			t.Fatalf("consent key %q escaped the modal", forbidden)
		}
	}
	step(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != "plan" || m.running {
		t.Fatal("Enter left the consent modal")
	}
	if _, ok := keyBindingForContext("/", "plan"); ok {
		t.Fatal("search must not be available on consent")
	}
	for _, binding := range keyMap {
		if containsContext(binding.contexts, contextPlan) && m.keyReason(binding, false) == "" {
			for _, key := range binding.keys {
				if !consentKeyAllowed(key) {
					t.Fatalf("unexpected modal key %q in shared key map", key)
				}
			}
		}
	}
	press(m, "b")
	if m.hex || m.section != 0 {
		t.Fatal("summary hex view was mistaken for the approved plan bytes")
	}
	press(m, "tab")
	if m.screen != "plan" || m.section != 1 || !bytes.Equal(m.doc.RawBytes(), preview) {
		t.Fatal("Tab did not open the exact retained plan in the content viewer")
	}
	if strings.Contains(m.View(), "byte pages") || strings.Contains(m.View(), "Read all byte") {
		t.Fatal("old byte-page consent wording remains")
	}
	press(m, "b")
	if !m.hex || !strings.Contains(m.View(), "00000000") {
		t.Fatal("b did not open the exact plan's byte view")
	}
	press(m, "?")
	if m.screen != "help" {
		t.Fatal("help is unavailable on consent")
	}
	press(m, "y")
	if m.running {
		t.Fatal("help overlay approved the plan")
	}
	press(m, "esc")
	if m.screen != "plan" || !bytes.Equal(m.preview, preview) {
		t.Fatal("closing help changed consent")
	}
	press(m, "esc")
	if m.screen != "examples" || m.preview != nil || m.digest != "" || m.running {
		t.Fatal("Escape did not deny and invalidate the exact plan")
	}
	press(m, "r")
	if m.screen != "plan" {
		t.Fatal("second preview did not open the modal")
	}
	press(m, "q")
	if m.preview != nil || m.digest != "" || m.running {
		t.Fatal("quitting did not deny and invalidate the exact plan")
	}
}

func consentKeyAllowed(key string) bool {
	allowed := map[string]bool{
		"q": true, "ctrl+c": true, "?": true, "esc": true, "tab": true, "shift+tab": true,
		"y": true, "n": true, "b": true, "j": true, "down": true, "k": true, "up": true,
		"pgdown": true, "pgup": true, "home": true, "g": true, "end": true, "G": true,
		"h": true, "left": true, "l": true, "right": true,
		// Emacs navigation aliases only move or leave; none act.
		"ctrl+g": true, "ctrl+n": true, "ctrl+p": true, "ctrl+d": true, "ctrl+u": true,
		"ctrl+f": true, "ctrl+b": true, "ctrl+v": true, "alt+v": true, "alt+<": true, "alt+>": true,
	}
	return allowed[key]
}

func newConsentActionModel(t *testing.T) (*Model, *Actions, evidence.SnapshotPair) {
	t.Helper()
	s, selection := setup(t, false)
	s.Close()
	a := &Actions{Project: selection.Project, Repetitions: 1, Limits: sandbox.Limits{Seconds: 10, OutputBytes: 4096}}
	m := New(t.Context(), selection, Jobs{Actions: a})
	t.Cleanup(m.Close)
	drain(m, m.Init())
	press(m, "r")
	if m.screen != "plan" || len(m.preview) == 0 {
		t.Fatal("could not prepare consent plan", m.status)
	}
	return m, a, selection.Pair
}

func TestConsentFallbackKeepsAndApprovesExactPreviewBytes(t *testing.T) {
	m, _, pair := newConsentActionModel(t)
	bad := bytes.Replace(m.preview, []byte(`"App": {`), []byte(`"App": {"future_field": true,`), 1)
	if bytes.Equal(bad, m.preview) {
		t.Fatal("test did not add an unknown nested sandbox field")
	}
	digest := consentTestDigestOf(bad)
	summary, summaryErr := ConsentSummary(bad)
	if summaryErr == nil {
		t.Fatal("malformed preview unexpectedly decoded")
	}
	_, cmd := m.Update(prepared{generation: m.generation, pair: pair, raw: bad, digest: digest, summary: summary, summaryErr: summaryErr})
	drain(m, cmd)
	if !m.summaryUnavailable || m.section != 0 || len(m.sections()) != 1 || m.sections()[0].Name != "Exact plan" {
		t.Fatal("malformed summary did not fall back to Exact plan", m.View())
	}
	if !bytes.Equal(m.preview, bad) || m.digest != digest || !strings.Contains(m.View(), "Summary unavailable") || !bytes.Equal(m.doc.RawBytes(), bad) {
		t.Fatal("fallback changed the exact approval bytes or omitted its limitation", m.View())
	}

	_, run := m.Update(key("y"))
	if !m.running || m.preview != nil || m.digest != "" {
		t.Fatal("y did not consume the retained preview for one approval")
	}
	batch, ok := run().(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatalf("approval did not dispatch the retained preview job: %#v", run)
	}
	result, ok := batch[0]().(ran)
	if !ok || result.err == nil || !strings.Contains(result.err.Error(), "saved execution plan no longer matches frozen inputs") {
		t.Fatalf("runner did not strictly decode the exact malformed preview: %#v", result)
	}
	m.Update(result)
	if m.running || len(m.activity) == 0 || !strings.Contains(m.activity[len(m.activity)-1].Detail, "saved execution plan no longer matches frozen inputs") {
		t.Fatal("malformed approved bytes did not fail closed or log the full error", m.status, m.activity)
	}
}

func TestConsentApprovalStillRequiresCurrentIdleBasis(t *testing.T) {
	m, _, pair := newConsentActionModel(t)
	raw := append([]byte(nil), m.preview...)
	digest := m.digest
	for _, busy := range []string{"run", "action"} {
		if busy == "run" {
			m.running = true
		} else {
			m.actionBusy = true
		}
		press(m, "y")
		if m.screen != "plan" || !bytes.Equal(m.preview, raw) || m.digest != digest {
			t.Fatalf("approval crossed active %s barrier", busy)
		}
		m.running, m.actionBusy = false, false
	}
	m.planPair.Candidate = consentTestDigest('f')
	press(m, "y")
	if m.screen != "plan" || !bytes.Equal(m.preview, raw) || m.running {
		t.Fatal("approval crossed the selected-pair barrier")
	}
	m.planPair = pair
	press(m, "n")
	if m.preview != nil || m.digest != "" || m.running {
		t.Fatal("denial retained approval")
	}
}
