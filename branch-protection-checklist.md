# Branch Protection Checklist (`main`)

Use this once after the repo is connected to GitHub.

## Repository Settings

- [ ] Default branch is `main`
- [ ] Auto-merge is enabled
- [ ] Automatically delete head branches is enabled

## Branch Protection Rule (`main`)

- [ ] Require a pull request before merging
- [ ] Require approvals (optional, set to your preference)
- [ ] Dismiss stale approvals when new commits are pushed (recommended)
- [ ] Require conversation resolution before merging
- [ ] Require status checks to pass before merging
- [ ] Require branches to be up to date before merging

### Required Status Checks

- [ ] `schema-validate`
- [ ] `lint`
- [ ] `unit-test`
- [ ] `race-test`
- [ ] `build`

## Recommended Additional Protections

- [ ] Restrict who can push to `main` (if team access grows)
- [ ] Do not allow force pushes
- [ ] Do not allow deletions of `main`
- [ ] Require signed commits (optional)

## GitHub Actions Permissions

- [ ] Actions are allowed to run for PRs and `main`
- [ ] Agent runtime account is authenticated with `gh auth status`

## Dry Run Validation

- [ ] Open a test PR with a docs-only change
- [ ] Confirm all required checks appear and pass
- [ ] Confirm PR cannot merge while a required check is pending/failing
- [ ] Confirm auto-merge works when checks pass
