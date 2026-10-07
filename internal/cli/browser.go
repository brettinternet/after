package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/brettinternet/after/internal/browser"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/gotestreport"
	"github.com/brettinternet/after/internal/sandbox"
	"github.com/brettinternet/after/internal/store"
	ucli "github.com/urfave/cli/v2"
)

func browseCommand(state *invocation, ctx *ucli.Context) error {
	if !state.tty || !validDigest(ctx.Args().First()) || !validDigest(ctx.String("base")) {
		return invalid("--tui requires a terminal, candidate ID and --base snapshot ID")
	}
	for _, flag := range []string{"select", "mode", "receipt", "accept", "reason"} {
		if ctx.IsSet(flag) {
			return invalid("TUI browsing cannot mutate a pin; use headless review")
		}
	}
	ids := ctx.StringSlice("evidence")
	if len(ids) > browser.MaxEvidence {
		return invalid("at most 32 evidence IDs")
	}
	selection := browser.Selection{Pair: evidence.SnapshotPair{Base: evidence.Digest(ctx.String("base")), Candidate: evidence.Digest(ctx.Args().First())}}
	for _, id := range ids {
		if !validDigest(id) {
			return invalid("invalid evidence ID")
		}
		selection.Evidence = append(selection.Evidence, evidence.Digest(id))
	}
	if ctx.IsSet("import-file") != ctx.IsSet("producer") || (ctx.IsSet("producer") && (strings.TrimSpace(ctx.String("producer")) == "" || len(ctx.String("producer")) > 256)) {
		return invalid("TUI import requires file and bounded caller producer")
	}
	cfg, err := configFlags(ctx)
	if err != nil {
		return err
	}
	selection.Project = cfg.Project
	actions := &browser.Actions{Project: cfg.Project, Repetitions: cfg.Repetitions, Limits: sandbox.Limits{Seconds: cfg.RunSeconds, OutputBytes: cfg.OutputBytes}, Docker: sandbox.Docker{Binary: cfg.DockerBinary, Host: cfg.DockerHost}}
	jobs := browser.Jobs{Actions: actions}
	if ctx.IsSet("import-file") {
		file, producer := ctx.String("import-file"), ctx.String("producer")
		jobs.Import = func(jobctx context.Context) (string, error) {
			if err := jobctx.Err(); err != nil {
				return "", err
			}
			input, err := openInput(file)
			if err != nil {
				return "", err
			}
			defer input.Close()
			report, err := gotestreport.Import(input, gotestreport.Metadata{Producer: producer, Snapshot: selection.Pair.Candidate, ImportedAt: time.Now().UTC()})
			if err != nil {
				return "", err
			}
			raw, err := json.Marshal(report)
			if err != nil {
				return "", err
			}
			s, err := actions.Store()
			if err != nil {
				return "", err
			}
			artifact, err := s.PutArtifact(raw, "report", store.MaxBlobBytes)
			return string(artifact.Content), err
		}
	}
	m := browser.New(state.ctx, selection, jobs)
	if err := browser.Run(m, state.reader, state.stdout); err != nil {
		return operational("terminal review failed")
	}
	if state.jsonOutput {
		_, err = fmt.Fprintln(state.stdout, string(m.SessionJSON()))
	}
	return err
}
