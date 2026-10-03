package cli

import (
	"strings"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/review"
	"github.com/brettinternet/after/internal/runner"
	"github.com/brettinternet/after/internal/sandbox"
	"github.com/brettinternet/after/internal/store"
	ucli "github.com/urfave/cli/v2"
)

func pinCommand(state *invocation) *ucli.Command {
	return &ucli.Command{Name: "pin", Usage: "pin a finite example or broader human intent against a stored receipt (no execution)", ArgsUsage: "<receipt-id>", Flags: append(commonFlags(),
		&ucli.StringFlag{Name: "expectation", Usage: "explicit human expectation (maximum 4096 bytes)"},
		&ucli.StringFlag{Name: "scope", Usage: "required finite_example or human_intent; neither asserts universal proof"},
		&ucli.StringFlag{Name: "reason", Usage: "required human reason"},
	), Action: func(ctx *ucli.Context) error {
		if err := requireArgs(ctx, 1); err != nil {
			return err
		}
		if !validDigest(ctx.Args().First()) || strings.TrimSpace(ctx.String("expectation")) == "" || !reviewReason(ctx.String("reason")) || len(ctx.String("expectation")) > 4096 || (ctx.String("scope") != string(evidence.FiniteExample) && ctx.String("scope") != string(evidence.HumanIntent)) {
			return invalid("pin requires receipt ID, expectation, explicit scope, and bounded reason")
		}
		cfg, err := configFlags(ctx)
		if err != nil {
			return err
		}
		s, err := store.Open(cfg.Project, true, nil)
		if err != nil {
			return operational("cannot open private evidence store")
		}
		defer s.Close()
		p, err := review.Create(s, evidence.Digest(ctx.Args().First()), ctx.String("expectation"), evidence.PinScope(ctx.String("scope")), ctx.String("reason"))
		if err != nil {
			return invalid("cannot pin this receipt with the requested scope")
		}
		return writeReview(state, s, p.ID)
	}}
}

func reviewReason(reason string) bool { return strings.TrimSpace(reason) != "" && len(reason) <= 4096 }

func reviewCommand(state *invocation) *ucli.Command {
	return &ucli.Command{Name: "review", Usage: "browse captured evidence with --tui, or inspect/mutate a headless pin revision", ArgsUsage: "<pin-revision-id> OR --tui <candidate-id> --base <base-id>", Flags: append(commonFlags(),
		&ucli.BoolFlag{Name: "tui", Usage: "browse immutable captures without execution (requires terminal)"},
		&ucli.StringFlag{Name: "base", Usage: "matching captured base snapshot for --tui"},
		&ucli.StringSliceFlag{Name: "evidence", Usage: "stored comparison, receipt or imported report ID for --tui (repeatable, maximum 32)"},
		&ucli.StringFlag{Name: "import-file", Usage: "file read only when i is pressed in --tui"},
		&ucli.StringFlag{Name: "producer", Usage: "caller provenance for the explicit TUI import action"},
		&ucli.StringFlag{Name: "select", Usage: "explicitly select a captured candidate snapshot ID"},
		&ucli.StringFlag{Name: "mode", Usage: "required with --select: original_base or last_inspected"},
		&ucli.StringFlag{Name: "receipt", Usage: "attach an already-authorized run receipt without accepting it"},
		&ucli.BoolFlag{Name: "accept", Usage: "record a separate human decision on the selected current receipt"},
		&ucli.StringFlag{Name: "reason", Usage: "required reason for every mutation"},
	), Action: func(ctx *ucli.Context) error {
		if err := requireArgs(ctx, 1); err != nil {
			return err
		}
		if ctx.Bool("tui") {
			return browseCommand(state, ctx)
		}
		for _, flag := range []string{"base", "evidence", "import-file", "producer"} {
			if ctx.IsSet(flag) {
				return invalid("browser flags require --tui")
			}
		}
		id := evidence.Digest(ctx.Args().First())
		if !validDigest(string(id)) {
			return invalid("invalid pin revision ID")
		}
		actions := 0
		for _, flag := range []string{"select", "receipt", "accept"} {
			if ctx.IsSet(flag) {
				actions++
			}
		}
		if actions > 1 || (ctx.IsSet("accept") && !ctx.Bool("accept")) || (actions > 0 && !reviewReason(ctx.String("reason"))) || (actions == 0 && ctx.IsSet("reason")) {
			return invalid("choose at most one review action, with a bounded reason")
		}
		if ctx.IsSet("select") {
			if !validDigest(ctx.String("select")) || (ctx.String("mode") != string(evidence.OriginalBase) && ctx.String("mode") != string(evidence.FollowUp)) {
				return invalid("selection requires snapshot ID and explicit comparison mode")
			}
		} else if ctx.IsSet("mode") {
			return invalid("mode requires --select")
		}
		if ctx.IsSet("receipt") && !validDigest(ctx.String("receipt")) {
			return invalid("invalid receipt ID")
		}
		cfg, err := configFlags(ctx)
		if err != nil {
			return err
		}
		s, err := store.Open(cfg.Project, actions > 0, nil)
		if err != nil {
			return operational("cannot open private evidence store")
		}
		defer s.Close()
		if actions == 0 {
			return writeReview(state, s, id)
		}
		var p evidence.Pin
		switch {
		case ctx.IsSet("select"):
			old, err := store.Get[evidence.Pin](s, id)
			if err != nil || old.Scope == "" {
				return invalid("pin revision unavailable or legacy scope is unknown")
			}
			pair := evidence.SnapshotPair{Base: old.BasisSnapshots.Base, Candidate: evidence.Digest(ctx.String("select"))}
			if evidence.ReviewMode(ctx.String("mode")) == evidence.FollowUp {
				pair.Base = old.History[len(old.History)-1].Review.Target.Snapshots.Candidate
			}
			for _, snapshot := range []evidence.Digest{pair.Base, pair.Candidate} {
				if _, err := store.Get[evidence.Snapshot](s, snapshot); err != nil {
					return invalid("selected snapshot is unavailable")
				}
			}
			target := evidence.ReviewBasis{Snapshots: pair}
			// Unsupported/incomplete captures remain inspectable and reopen unknown.
			// Prepare only reads source; it never contacts Docker or builds a project.
			plan, prepareErr := runner.Prepare(s, pair, cfg.Repetitions, sandbox.Limits{Seconds: cfg.RunSeconds, OutputBytes: cfg.OutputBytes})
			if prepareErr == nil {
				target = plan.ReviewBasis()
			}
			p, err = review.Select(s, id, target, evidence.ReviewMode(ctx.String("mode")), ctx.String("reason"))
		case ctx.IsSet("receipt"):
			p, err = review.Attach(s, id, evidence.Digest(ctx.String("receipt")), ctx.String("reason"))
		case ctx.Bool("accept"):
			p, err = review.Accept(s, id, ctx.String("reason"))
		}
		if err != nil {
			return invalid("review action rejected: unavailable/mismatched basis, incomplete evidence, or history limit")
		}
		return writeReview(state, s, p.ID)
	}}
}

func writeReview(state *invocation, s *store.Store, id evidence.Digest) error {
	v, err := review.Inspect(s, id)
	if err != nil {
		return invalid("pin revision or its bound evidence is unavailable")
	}
	return writeJSON(state, "review", v)
}
