package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/brettinternet/after/internal/browser"
	"github.com/brettinternet/after/internal/gotestreport"
	"github.com/brettinternet/after/internal/sandbox"
	"github.com/brettinternet/after/internal/store"
	ucli "github.com/urfave/cli/v2"
)

func browseCommand(state *invocation, ctx *ucli.Context) error {
	cfg, err := configFlags(ctx)
	if err != nil {
		return err
	}
	if !state.tty {
		return invalidWithFix("review requires a terminal", "run after review BASE CANDIDATE in a terminal")
	}
	args := ctx.Args().Slice()
	selection, err := resolveBrowserSelection(cfg.Project, args[0], args[1], args[2:])
	if err != nil {
		return err
	}
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
