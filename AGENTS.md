# Working on AFTER

Read [docs/HANDOFF.md](docs/HANDOFF.md), [docs/IMPLEMENTATION.md](docs/IMPLEMENTATION.md), and the complete selected Backlog.md task before implementation. The proposal is not an implemented engine. Go is the product language; Bun/Playwright are documentation tooling only.

## Tools and checks

- Install tools/hooks with `mise trust && mise install && mise exec -- task init`. Use `mise exec -- task <target>`; do not reconstruct commands.
- Use the smallest applicable checks. Stage intended files and run `mise exec -- task check:staged` before each commit. Run `task check` for cross-project work or the release gate, not every edit.
- `task test` currently checks local links and backlog integrity. AFTER-1 must add real Go tests/build/vet/format checks to Task and CI.
- Presentation changes require `task docs:check` in actual Chromium; `task docs:render` also regenerates committed PDF/preview. Install Chromium with `task docs:browser:install` when needed.
- Prefer simple ordinary Go packages, explicit argv, bounded data/processes and focused tests. No plugin framework, model dependency or frontend app for the first slice.

## Backlog is the source of truth

Use the pinned `mise exec -- backlog` CLI, never direct task/metadata edits or formatter rewrites. Read `backlog instructions overview` and the matching creation/execution/finalization guide. Search existing tasks before creating work.

1. From the primary checkout on `main`, inspect `backlog task list --labels poc --status 'To Do' --json`. Read the complete task and its dependencies. Choose ready M1/M2 work; priority then ID breaks ties. Do not automatically start the human-required/follow-up milestone.
2. Recheck status, set the selected task to `In Progress` with your assignee, and reread to confirm **before** creating an implementation worktree. An In Progress task is claimed, not available; resume it only on explicit handoff.
3. Backlog.md's status edit is not an atomic distributed lock. Default to one orchestrator and one primary-checkout writer. Do not start concurrent autonomous claimers; use explicit coordination if the operator requests parallel work.
4. Keep all Backlog state changes in the primary checkout, including plans, acceptance checks, notes and final summaries. A worktree's copied backlog is not authoritative and must not be edited.
5. After researching current code, record the bounded implementation plan via CLI. Record actual command results and limitations as work progresses. Acceptance criteria are observable outcomes; DoD is separate completion hygiene.
6. A real blocker gets exact evidence, a bounded next action and the objective unblock condition in task notes; ask the operator for required credentials/decisions/environment. Never weaken execution isolation or invent evidence to finish.
7. Integrate verified implementation into main, then mark criteria/DoD and Done from primary using CLI and reread. Done requires actual tests and delivery evidence, not just passing compilation or a plausible screenshot.

## Git delivery and worktrees

- Preserve pre-existing changes. Bootstrap/docs may be maintained on main; agent-created implementation branches must use `.worktrees/`.
- Inspect existing worktrees before creating another. Read/approve `.config/wt.toml`, then use `mise exec worktrunk -- wt switch --create <branch> --base main --no-cd --format=json`. Work in the returned path.
- Hooks install the pinned toolchain, frozen documentation dependencies and Lefthook. They do not copy ignored files/secrets or start services. Plain Git worktrees require `mise exec -- task setup:worktree`.
- One implementation writer per checkout. Keep the primary task claim while implementation happens in the worktree. Commit only implementation files there; never reset main to clear the claim.
- Before integration, reread the authoritative claim and main's state. With one integrator, fast-forward the tested implementation branch when possible; if main advanced, rebase/merge in the owned worktree and rerun affected checks. Preserve primary task edits; stop on an unexpected conflict.
- Commit final task metadata from primary after integration. A request for one task does not authorize the next task or a push. Push only when the current operator explicitly requests it; never open a PR without permission.
- Before cleanup, verify the checkout/branch against the session creation receipt, check active agents and unsaved panes, and use Worktrunk removal from a surviving checkout so required hooks run. Missing/ambiguous ownership means retain and report, never guess.
- Use `gh` for GitHub operations. Never commit private .after data, credentials, local paths or research participant data.

## Product invariants

No project execution on open/import/inspect. Builds are execution too. A worktree is not a sandbox. A changed snapshot does not create an observed result. A changed oracle does not establish preserved behavior. Terminal and repository content is untrusted. Evidence applicability, execution outcome and human acceptance are separate. The raw diff and unknown inventory remain usable when everything richer fails.
