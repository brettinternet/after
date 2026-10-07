package cli

import (
	"errors"
	"os"
	"strings"

	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/review"
	"github.com/brettinternet/after/internal/runner"
	"github.com/brettinternet/after/internal/sandbox"
	"github.com/brettinternet/after/internal/store"
	ucli "github.com/urfave/cli/v2"
)

func pinCommand(state *invocation) *ucli.Command {
	return &ucli.Command{Name: "pin", Usage: "create, inspect, or update a human expectation (no execution)", ArgsUsage: "<receipt-or-pin-id>", Before: outputBefore(state), Flags: append(globalFlags(),
		&ucli.StringFlag{Name: "expectation", Usage: "human expectation to pin (maximum 4096 bytes)"},
		&ucli.StringFlag{Name: "scope", Value: string(evidence.FiniteExample), Usage: "finite_example or human_intent; neither asserts universal proof"},
		&ucli.BoolFlag{Name: "accept", Usage: "accept the currently reviewed complete result"},
		&ucli.StringFlag{Name: "attach", Usage: "attach an already-authorized receipt without accepting it"},
		&ucli.StringFlag{Name: "select", Usage: "select a captured candidate snapshot"},
		&ucli.StringFlag{Name: "mode", Value: string(evidence.OriginalBase), Usage: "comparison base: original_base or last_inspected"},
		&ucli.StringFlag{Name: "reason", Usage: "verbatim human reason for the pin history event"},
	), Action: func(ctx *ucli.Context) error {
		if err := requireArgs(ctx, 1); err != nil {
			return err
		}
		create := ctx.IsSet("expectation")
		actions := 0
		for _, name := range []string{"accept", "attach", "select"} {
			if ctx.IsSet(name) {
				actions++
			}
		}
		if actions > 1 || (ctx.IsSet("accept") && !ctx.Bool("accept")) {
			return invalidWithFix("choose at most one pin action", "use exactly one of --accept, --attach RECEIPT, or --select SNAPSHOT")
		}
		if create {
			if actions > 0 || ctx.IsSet("mode") {
				return invalidWithFix("pin creation cannot be combined with a pin decision", "use after pin RECEIPT --expectation TEXT")
			}
			if strings.TrimSpace(ctx.String("expectation")) == "" || len(ctx.String("expectation")) > 4096 {
				return invalidWithFix("expectation must contain 1 to 4096 bytes", "use after pin RECEIPT --expectation TEXT")
			}
			scope := evidence.PinScope(ctx.String("scope"))
			if scope != evidence.FiniteExample && scope != evidence.HumanIntent {
				return invalidWithFix("scope must be finite_example or human_intent", "use --scope finite_example or --scope human_intent")
			}
			if ctx.IsSet("reason") && !reviewReason(ctx.String("reason")) {
				return invalidWithFix("reason must contain 1 to 4096 bytes", "provide a non-empty --reason TEXT or omit --reason")
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
			receiptID, err := resolveID(s, ctx.Args().First(), "receipt")
			if err != nil {
				return err
			}
			reason := reasonOrDefault(ctx, "Pinned from the command line")
			p, err := review.Create(s, receiptID, ctx.String("expectation"), scope, reason)
			if err != nil {
				if strings.Contains(err.Error(), "finite example requires a complete observed result") {
					return invalidWithFix("finite_example requires a complete observed result", "use --scope human_intent or pin a complete observed receipt")
				}
				return invalidWithFix("receipt cannot support this pin", "use a runner receipt with concrete scenario bindings")
			}
			return writeReview(state, s, p.ID)
		}
		if actions == 0 {
			if ctx.IsSet("scope") || ctx.IsSet("mode") || ctx.IsSet("reason") {
				return invalidWithFix("pin options require an expectation or decision", "use after pin PIN to inspect a pin, or add --expectation TEXT to create one")
			}
			cfg, err := configFlags(ctx)
			if err != nil {
				return err
			}
			s, err := store.Open(cfg.Project, false, nil)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return noIDMatch(ctx.Args().First(), "pin")
				}
				return operational("cannot open private evidence store for reading")
			}
			defer s.Close()
			id, err := resolveID(s, ctx.Args().First(), "pin")
			if err != nil {
				return err
			}
			return writeReview(state, s, id)
		}
		if ctx.IsSet("scope") || ctx.IsSet("expectation") {
			return invalidWithFix("pin decisions cannot change the expectation or scope", "use after pin PIN --accept, --attach RECEIPT, or --select SNAPSHOT")
		}
		if ctx.IsSet("mode") && !ctx.IsSet("select") {
			return invalidWithFix("--mode requires --select", "use after pin PIN --select SNAPSHOT [--mode original_base|last_inspected]")
		}
		if ctx.IsSet("reason") && !reviewReason(ctx.String("reason")) {
			return invalidWithFix("reason must contain 1 to 4096 bytes", "provide a non-empty --reason TEXT or omit --reason")
		}
		cfg, err := configFlags(ctx)
		if err != nil {
			return err
		}
		s, err := store.Open(cfg.Project, true, nil)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return noIDMatch(ctx.Args().First(), "pin")
			}
			return operational("cannot open private evidence store")
		}
		defer s.Close()
		id, err := resolveID(s, ctx.Args().First(), "pin")
		if err != nil {
			return err
		}
		var selected, receipt evidence.Digest
		if ctx.IsSet("select") {
			selected, err = resolveID(s, ctx.String("select"), "snapshot")
			if err != nil {
				return err
			}
		}
		if ctx.IsSet("attach") {
			receipt, err = resolveID(s, ctx.String("attach"), "receipt")
			if err != nil {
				return err
			}
		}
		if ctx.IsSet("mode") && ctx.String("mode") != string(evidence.OriginalBase) && ctx.String("mode") != string(evidence.FollowUp) {
			return invalidWithFix("mode must be original_base or last_inspected", "use --mode original_base or --mode last_inspected")
		}
		reason := ""
		var p evidence.Pin
		switch {
		case ctx.IsSet("select"):
			old, err := store.Get[evidence.Pin](s, id)
			if err != nil || old.Scope == "" {
				return invalidWithFix("pin revision or its scope is unavailable", "use a stored pin revision ID from after pin")
			}
			pair := evidence.SnapshotPair{Base: old.BasisSnapshots.Base, Candidate: selected}
			mode := evidence.ReviewMode(ctx.String("mode"))
			if mode == evidence.FollowUp {
				pair.Base = old.History[len(old.History)-1].Review.Target.Snapshots.Candidate
			}
			for _, snapshot := range []evidence.Digest{pair.Base, pair.Candidate} {
				if _, err := store.Get[evidence.Snapshot](s, snapshot); err != nil {
					return invalidWithFix("selected snapshot is unavailable", "select a snapshot ID from this private store")
				}
			}
			target := evidence.ReviewBasis{Snapshots: pair}
			// Unsupported/incomplete captures remain inspectable and reopen unknown.
			// Prepare reads source only; it never contacts Docker or builds a project.
			plan, prepareErr := runner.Prepare(s, pair, cfg.Repetitions, sandbox.Limits{Seconds: cfg.RunSeconds, OutputBytes: cfg.OutputBytes})
			if prepareErr == nil {
				target = plan.ReviewBasis()
			}
			reason = reasonOrDefault(ctx, "Selected from the command line")
			p, err = review.Select(s, id, target, mode, reason)
		case ctx.IsSet("attach"):
			reason = reasonOrDefault(ctx, "Attached from the command line")
			p, err = review.Attach(s, id, receipt, reason)
		case ctx.Bool("accept"):
			reason = reasonOrDefault(ctx, "Accepted from the command line")
			p, err = review.Accept(s, id, reason)
		}
		if err != nil {
			return invalidWithFix("pin decision was rejected: the basis or evidence is unavailable, mismatched, or incomplete", "inspect the pin with after pin PIN and select a compatible current receipt or snapshot")
		}
		return writeReview(state, s, p.ID)
	}}
}

func reasonOrDefault(ctx *ucli.Context, fallback string) string {
	if ctx.IsSet("reason") {
		return ctx.String("reason")
	}
	return fallback
}

func reviewReason(reason string) bool { return strings.TrimSpace(reason) != "" && len(reason) <= 4096 }

func reviewCommand(state *invocation) *ucli.Command {
	return &ucli.Command{Name: "review", Usage: "browse a captured snapshot pair in the terminal review", ArgsUsage: "<base-snapshot-id> <candidate-snapshot-id> [evidence-id ...]", Before: outputBefore(state), Flags: append(globalFlags(),
		&ucli.StringFlag{Name: "import-file", Usage: "file to import only after the explicit TUI import action"},
		&ucli.StringFlag{Name: "producer", Usage: "required caller provenance for the explicit TUI import action"},
	), Action: func(ctx *ucli.Context) error {
		if ctx.NArg() < 2 {
			return requireArgs(ctx, 2)
		}
		if len(ctx.Args().Slice())-2 > 32 {
			return invalidWithFix("review accepts at most 32 evidence IDs", "use after review BASE CANDIDATE with no more than 32 evidence IDs")
		}
		if ctx.IsSet("import-file") != ctx.IsSet("producer") || (ctx.IsSet("producer") && (strings.TrimSpace(ctx.String("producer")) == "" || len(ctx.String("producer")) > 256)) {
			return invalidWithFix("TUI import requires a file and bounded caller provenance", "use after review BASE CANDIDATE --import-file FILE --producer TEXT")
		}
		return browseCommand(state, ctx)
	}}
}

func writeReview(state *invocation, s *store.Store, id evidence.Digest) error {
	v, err := review.Inspect(s, id)
	if err != nil {
		return invalidWithFix("pin revision or its bound evidence is unavailable", "use a stored pin revision ID from after pin")
	}
	if state.jsonOutput || state.forceJSON {
		return writeResult(state, "review", v)
	}
	heads, headErr := review.NewerHeads(s, v.Pin)
	return writeResult(state, "review", reviewInspection{View: v, NewerHeads: heads, HeadsUnavailable: headErr != nil})
}

type reviewInspection struct {
	review.View
	NewerHeads       []evidence.Digest `json:"-"`
	HeadsUnavailable bool              `json:"-"`
}
