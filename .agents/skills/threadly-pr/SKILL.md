---
name: threadly-pr
description: Prepare, validate, and publish a clear draft pull request for the Threadly repository. Use when Codex needs to inspect a Threadly change, select the relevant api/front checks, write a Japanese PR description with purpose, cause, actions, results, scope, verification, and review focus, or commit, push, and open the draft PR.
---

# Threadly PR作成

Threadlyの変更を、レビュワーが目的・理由・確認結果を短時間で理解できるPull Requestへ整える。PRはコード提出の付属物ではなく、後から変更意図を追える開発文書として扱う。

## Workflow

### 1. Scope and context

1. Read `git status -sb`, the staged and unstaged diff, the current branch, and the `origin` remote before editing or staging anything.
2. Treat only the requested files as in scope. If the worktree contains unrelated changes, stop before staging and ask which paths belong in the PR.
3. Read repository guidance such as `AGENTS.md`, `.github/copilot-instructions.md`, `README.md`, and the nearest package instructions. Keep review comments and the PR body in Japanese when the repository guidance requires it.
4. Use available project notes and design context when they affect the change, but never copy personal-vault details into the repository or the external PR.
5. Describe the actual diff, not the requested intention. If the change is documentation or a skill, say so explicitly and do not invent runtime behavior.

Threadly-specific defaults:

- `api/` is the Go Gin/GORM/MySQL application and uses `go test ./...` through `api/Makefile`.
- `front/` is a Vite/TypeScript application. Run its lint/build checks only when the diff touches `front/` or shared API artifacts.
- Existing Threadly API design context treats `front/` changes as out of scope for API-only work. Keep that boundary visible in the PR.
- Do not claim Docker/MySQL integration coverage when only local unit/package tests ran.

### 2. Pre-PR checks

Run checks appropriate to the changed paths and record exact results:

- Run `git diff --check` and inspect the complete diff for accidental files, stale comments, debug output, typos, and generated artifacts.
- For `api/` changes, run `make -C api test` (or `go test ./...` from `api/`). Run `go vet ./...` when the environment supports it and the change is code-facing.
- For `front/` changes, run `npm run lint` and `npm run build` from `front/` when dependencies are available.
- For generated API clients or Swagger changes, verify that the generated files match the source change and state the generation command used.
- Check the branch against the intended base, normally `origin/main`. Report whether it is behind or conflicts are present; do not silently rebase, merge, or resolve conflicts.
- Separate passed checks, failed checks, skipped checks, and environment blockers. Never present a skipped check as passed.

If a test prints `error` or `warn`, determine whether it is an expected test fixture message. Mention unresolved warnings in the PR under `課題` or `注意事項`.

### 3. Write the PR

Read [references/pr-body-template.md](references/pr-body-template.md), then fill every section with evidence from the diff and checks. Use concise Japanese prose and real Markdown newlines.

Required sections:

- `概要`: what changed and the user/developer impact.
- `背景・原因`: why the change was needed; for non-fixes, state `該当なし` and explain the motivation instead.
- `やったこと`: short, concrete bullets.
- `変更結果`: observable behavior, files, generated output, screenshots, or command results. Do not add a video for a change that is adequately shown by text.
- `やらないこと`: explicit out-of-scope items, especially `front/` for API-only work.
- `注意事項`: migration, startup, configuration, reviewer setup, or post-merge steps.
- `確認手順`: reproducible commands and manual steps, including prerequisites such as Docker/MySQL.
- `課題・特に見てほしい箇所`: unresolved concerns and focused review questions. Do not hide a known limitation.
- `備考`: related issues, designs, generated files, or other context.

Choose a title that states the result, not a vague activity. Keep the PR focused; split independent purposes instead of hiding them in one description. Default to a Draft PR so unfinished work cannot be merged accidentally.

### 4. Commit, publish, and verify

Only perform the publish flow when the user asked to create the PR and the scope is unambiguous.

1. Require `gh --version` and `gh auth status` before committing or pushing. If `gh` is unavailable or unauthenticated, stop the publish flow and report the exact blocker.
2. Stage explicit paths only. Use the project `commit-message-ja` skill when it is available; use an English Conventional Commit prefix and a Japanese, intent-focused subject/body.
3. Run `git diff --cached --check`, then commit and record the commit SHA.
4. Push the current branch with tracking using `git push -u origin <branch>`.
5. Prefer the connected GitHub PR tool after the push. If it cannot resolve the repository/head/base cleanly, use `gh pr create --draft` with a temporary Markdown body file containing real newlines.
6. Use the repository default branch as the base unless the user specifies another target. Do not assign reviewers or change labels unless requested or required by repository policy.
7. After creation, verify the PR URL, title, draft state, base/head branches, changed files, CI status, and conflict status. Report anything still waiting for review or checks.

When the publish prerequisite fails, still leave the validated local skill and PR body ready, but do not imply that a commit, push, or PR was created.

## Output contract

Report:

- changed paths and scope decision;
- checks run with pass/fail/skipped status;
- PR title and body summary;
- commit, branch, push, and PR URL only when each operation actually succeeded;
- blockers and the precise next command or user action when publishing could not complete.
