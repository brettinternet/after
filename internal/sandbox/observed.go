package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// Experiment separates the candidate's processes, files, scratch and output
// from the observer. Only an offline loopback namespace is shared. The caller
// must supply a trusted observer, never source from the candidate checkout.
type Experiment struct{ app, observer *Plan }

func PrepareExperiment(app, observer *Plan) (*Experiment, error) {
	if app == nil || observer == nil {
		return nil, errors.New("two plans required")
	}
	return &Experiment{app, observer}, nil
}
func (p *Experiment) Preview() ([]byte, string) {
	a, _ := p.app.Preview()
	o, _ := p.observer.Preview()
	b, _ := json.MarshalIndent(struct {
		App, Observer json.RawMessage
		Topology      string
	}{a, o, "app network=none; observer joins app network only; separate PID, IPC, filesystem and tmpfs; no host ports; app terminated after observation"}, "", "  ")
	return b, digest(b)
}

type ExperimentResult struct {
	App      Result `json:"app"`
	Observer Result `json:"observer"`
}

func (d Docker) Observe(ctx context.Context, p *Experiment, approved string) (r ExperimentResult, err error) {
	if p == nil {
		return r, ErrConsent
	}
	_, id := p.Preview()
	if approved != id {
		return r, ErrConsent
	}
	_, aid := p.app.Preview()
	_, oid := p.observer.Preview()
	r.App, err = d.execute(ctx, p.app, aid, "", func(ctx context.Context, name string) error {
		var e error
		r.Observer, e = d.execute(ctx, p.observer, oid, name, nil)
		return e
	})
	return r, err
}

func (d Docker) serve(ctx context.Context, config, name string, p *Plan, r Result, observe func(context.Context, string) error) (result Result, err error) {
	result = r
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	out := &output{max: p.spec.Limits.OutputBytes, cancel: cancel}
	done := make(chan struct{})
	go func() {
		d.command(child, config, nil, out, "start", "--attach", name)
		close(done)
		cancel() // An exited app cannot leave an observer waiting for readiness.
	}()
	// Join the attach goroutine on every exit; cancellation of the CLI is followed
	// by the outer exact-owner cleanup, which kills the entire app namespace.
	defer func() {
		cancel()
		<-done
		result.Output = out.b.String()
		result.Truncated = out.truncated
		if out.truncated {
			err = errors.Join(err, ErrOutput)
		}
		if ctx.Err() != nil {
			err = errors.Join(err, ctx.Err())
		}
	}()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		b, e := d.query(child, config, nil, "inspect", "--format", "{{.State.Running}}", name)
		if e != nil {
			return r, e
		}
		if string(b) == "true\n" {
			break
		}
		select {
		case <-done:
			return r, errors.New("app exited before observer startup")
		case <-child.Done():
			return r, child.Err()
		case <-ticker.C:
		}
	}
	err = observe(child, name)
	// The service must still be running at completion; its intentional teardown
	// is not a successful application exit and is recorded with exit_code=-1.
	select {
	case <-done:
		if ctx.Err() == nil {
			err = errors.New("app exited during observation")
		}
	default:
	}
	return r, err
}
