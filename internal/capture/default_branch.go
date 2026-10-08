package capture

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

// DefaultBranch is a locally available default-branch comparison. Reference is
// safe to pass to the capture CLI; Name is its short display name.
type DefaultBranch struct {
	Reference string
	Name      string
	Ahead     int
}

// FindDefaultBranch resolves origin/HEAD, then local main, then local master,
// and counts commits reachable from HEAD but not the selected branch. It uses
// the same isolated, bounded Git runner as capture and never contacts a remote.
func FindDefaultBranch(ctx context.Context, dir string) (DefaultBranch, bool, error) {
	var branch DefaultBranch
	var branchCommit string
	remote, err := git(ctx, dir, nil, "symbolic-ref", "-q", "refs/remotes/origin/HEAD")
	if err == nil {
		remoteRef := strings.TrimSpace(string(remote))
		const remotePrefix = "refs/remotes/origin/"
		name := strings.TrimPrefix(remoteRef, remotePrefix)
		if strings.HasPrefix(remoteRef, remotePrefix) && name != "" && name != "HEAD" {
			if commit, resolveErr := resolveCommit(ctx, dir, remoteRef); resolveErr == nil {
				branchCommit = commit
				branch.Name = name
				branch.Reference = "origin/" + name
				if localCommit, localErr := resolveCommit(ctx, dir, "refs/heads/"+name); localErr == nil && localCommit == commit {
					branch.Reference = name
				}
			}
		}
	}
	if ctx.Err() != nil {
		return DefaultBranch{}, false, ctx.Err()
	}
	if branchCommit == "" {
		for _, name := range []string{"main", "master"} {
			if commit, resolveErr := resolveCommit(ctx, dir, "refs/heads/"+name); resolveErr == nil {
				branch = DefaultBranch{Reference: name, Name: name}
				branchCommit = commit
				break
			}
			if ctx.Err() != nil {
				return DefaultBranch{}, false, ctx.Err()
			}
		}
	}
	if branchCommit == "" {
		return DefaultBranch{}, false, nil
	}
	head, err := resolveCommit(ctx, dir, "HEAD")
	if err != nil {
		if ctx.Err() != nil {
			return DefaultBranch{}, false, ctx.Err()
		}
		return DefaultBranch{}, false, nil
	}
	count, err := git(ctx, dir, nil, "rev-list", "--count", branchCommit+".."+head)
	if err != nil {
		return DefaultBranch{}, false, err
	}
	branch.Ahead, err = strconv.Atoi(strings.TrimSpace(string(count)))
	if err != nil || branch.Ahead < 0 {
		return DefaultBranch{}, false, err
	}
	return branch, true, nil
}

func resolveCommit(ctx context.Context, dir, ref string) (string, error) {
	output, err := git(ctx, dir, nil, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
	if err != nil {
		return "", err
	}
	commit := strings.TrimSpace(string(output))
	if !validOID(commit) {
		return "", errors.New("invalid resolved commit")
	}
	return commit, nil
}
