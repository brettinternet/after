package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/brettinternet/after/internal/browser"
	"github.com/brettinternet/after/internal/capture"
	"github.com/brettinternet/after/internal/config"
	"github.com/brettinternet/after/internal/evidence"
	"github.com/brettinternet/after/internal/gotestreport"
	"github.com/brettinternet/after/internal/sandbox"
	"github.com/brettinternet/after/internal/store"
	ucli "github.com/urfave/cli/v2"
)

func runBrowser(state *invocation, ctx *ucli.Context, cfg config.Config, selection browser.Selection, session browser.ReviewSession, captureOnStart, saveSession bool) error {
	if !state.tty || !state.stderrTTY {
		return invalidWithFix("review requires a terminal", fmt.Sprintf("run after inspect %s %s --json to inspect this pair", selection.Pair.Base, selection.Pair.Candidate))
	}
	actions := &browser.Actions{Project: cfg.Project, CaptureOptions: session.CaptureOptions(), Repetitions: cfg.Repetitions, Limits: sandbox.Limits{Seconds: cfg.RunSeconds, OutputBytes: cfg.OutputBytes}, Docker: sandbox.Docker{Binary: cfg.DockerBinary, Host: cfg.DockerHost}}
	jobs := browser.Jobs{Actions: actions, CaptureOnStart: captureOnStart}
	if ctx.IsSet("import-file") {
		file, producer := ctx.String("import-file"), ctx.String("producer")
		jobs.Import = func(jobctx context.Context, selected browser.Selection) (string, error) {
			if err := jobctx.Err(); err != nil {
				return "", err
			}
			input, err := openInput(file)
			if err != nil {
				return "", err
			}
			defer input.Close()
			report, err := gotestreport.Import(input, gotestreport.Metadata{Producer: producer, Snapshot: selected.Pair.Candidate, ImportedAt: time.Now().UTC()})
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
	var persistSession func(browser.ReviewSession) error
	if saveSession {
		persistSession = func(updated browser.ReviewSession) error { return actions.SaveReviewSession(updated) }
	}
	m.SetReviewSession(session, persistSession)
	err := browser.Run(m, state.reader, state.stderr)
	if err != nil {
		return operational("terminal review failed")
	}
	finalSession := m.ReviewSession()
	if saveSession {
		if err := saveReviewSessionFile(cfg.Project, finalSession); err != nil {
			return operational("cannot save private review session")
		}
		_, _ = fmt.Fprintf(state.stderr, "Saved review %s → %s · after review resumes it\n", shortID(finalSession.Pair.Base), shortID(finalSession.Pair.Candidate))
	}
	if state.jsonOutput {
		return writeResult(state, "review_session", finalSession)
	}
	return nil
}

func saveReviewSessionFile(project string, session browser.ReviewSession) error {
	raw, err := browser.MarshalReviewSession(session)
	if err != nil {
		return err
	}
	s, err := store.Open(project, true, nil)
	if err != nil {
		return err
	}
	defer s.Close()
	return s.WriteReviewSession(raw)
}

func captureForReview(state *invocation, project string, options capture.Options) (evidence.SnapshotPair, error) {
	s, err := store.Open(project, true, nil)
	if err != nil {
		return evidence.SnapshotPair{}, operational("cannot open private evidence store")
	}
	defer s.Close()
	stopNotice := startElapsedNotice(state, "review capture", time.Second)
	defer stopNotice()
	result, err := capture.Capture(state.ctx, project, s, options)
	if err != nil {
		return evidence.SnapshotPair{}, captureFailure(err)
	}
	return evidence.SnapshotPair{Base: result.Base.ID, Candidate: result.Candidate.ID}, nil
}
