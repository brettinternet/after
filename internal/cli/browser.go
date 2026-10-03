package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"

	"github.com/brettinternet/after/internal/browser"
	"github.com/brettinternet/after/internal/evidence"
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
	// Reuse the exact headless capture/import contracts, including bounded JSON
	// output and persistence. These callbacks run only on an explicit key action.
	job := func(args []string) browser.Job {
		return func(jobctx context.Context) (string, error) {
			var out, diagnostic bytes.Buffer
			code := run(jobctx, args, &out, &diagnostic, strings.NewReader(""), false)
			if code != ExitOK {
				return out.String(), errors.New("headless operation failed; " + diagnostic.String())
			}
			return out.String(), nil
		}
	}
	jobs := browser.Jobs{Capture: job([]string{"capture", "--project", cfg.Project})}
	if ctx.IsSet("import-file") {
		jobs.Import = job([]string{"import", "--project", cfg.Project, "--producer", ctx.String("producer"), "--snapshot", string(selection.Pair.Candidate), "--", ctx.String("import-file")})
	}
	m := browser.New(state.ctx, selection, jobs)
	if err := browser.Run(m, state.reader, state.stdout); err != nil {
		return operational("terminal review failed")
	}
	return nil
}
