---
name: threadly-pr
description: Prepare, validate, and publish clear draft pull requests or Stack PRs for the Threadly repository. Use when Codex needs to inspect a Threadly change, split a large dependent change into reviewable branch layers, select the relevant api/front checks, write Japanese PR descriptions with purpose, cause, actions, results, scope, verification, and review focus, or commit, push, and open draft PRs.
---

# Threadly PR作成

Threadlyの変更を、レビュワーが目的・理由・影響・確認結果を短時間で理解できるPull Requestへ整える。PRはコード提出の付属物ではなく、後から変更意図と検証根拠を追える開発文書として扱う。

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

### Stack PRを設計する

依存関係のある大きな変更を、各層ごとにレビューしたい場合はStack PRを使う。無関係な変更をまとめる手段には使わない。単一PRで目的・影響・レビュー観点を十分に説明でき、独立してマージできるなら通常のPRを優先する。

1. コミット前に、依存される順に各層の「目的、対象パス、検証、PR base」を決める。下位層は`main`、以降の層は直下のStack branchをbaseにする。
2. 1ブランチは1つのレビュー可能な責務に限定する。Threadlyの例では、`ドメイン/認証コア → 永続化・HTTP API → Post認可とAPI契約 → front/利用側` の順に積む。
3. API-only層には`front/`、Orval生成物、画面変更を入れない。API契約に対応するSwaggerは、その契約を変更するAPI層に入れる。フロント変更が作業ツリーに残る場合は、API層をコミット後に最上段のUI branchを作り、後続のPRへ隔離する。
4. 各層は下位層だけを前提にビルド・テストできる状態にする。必要なら一時worktreeでそのコミットをcheckoutして確認する。
5. Stackを作成・変更するときは、`gh-stack` Skillの非対話ルールに従う。特に`gh stack view --json`、`gh stack init --base <trunk> <branch...>`、`gh stack add <branch>`、`gh stack submit --auto --remote origin`を使い、引数なしの対話コマンドを実行しない。

### 2. Pre-PR checks

Run checks appropriate to the changed paths and record exact results:

- Run `git diff --check` and inspect the complete diff for accidental files, stale comments, debug output, typos, and generated artifacts.
- For `api/` changes, run `make -C api test` (or `go test ./...` from `api/`). Run `go vet ./...` when the environment supports it and the change is code-facing.
- For `front/` changes, run `npm run lint` and `npm run build` from `front/` when dependencies are available.
- For generated API clients or Swagger changes, verify that the generated files match the source change and state the generation command used.
- Check the branch against the intended base, normally `origin/main`. Report whether it is behind or conflicts are present; do not silently rebase, merge, or resolve conflicts.
- Stack PRでは、各branchのbaseが直下の層であること、`gh stack view --json` の `needsRebase` がすべて`false`であることを確認する。各PRの差分は、Stack全体ではなく直下のbaseとの差分で確認する。
- Separate passed checks, failed checks, skipped checks, and environment blockers. Never present a skipped check as passed.

If a test prints `error` or `warn`, determine whether it is an expected test fixture message. Mention unresolved warnings in the PR under `課題` or `注意事項`.

### 3. Write the PR

Read [references/pr-body-template.md](references/pr-body-template.md), then fill every section with evidence from the diff and checks. Use concise Japanese prose and real Markdown newlines.

本文は、次の順でレビューに必要な情報を渡す。

1. `概要`の冒頭1〜2文で、利用者または開発者にとっての結果（What）と目的（Why）を先に述べる。
2. `やったこと`で、実装方針（How）を具体的なendpoint、境界、データ、生成物の単位で示す。`その他`や`もろもろ`のような曖昧な表現を使わない。
3. `変更結果`と`影響範囲`で、観測できる振る舞い、互換性、設定・migration・利用側への影響を区別して書く。
4. `確認手順`には、実行したコマンドと実際の結果を記す。未実施・環境依存・失敗した確認は、理由とともに`注意事項`または`課題`へ記す。
5. `課題・特に見てほしい箇所`には、設計判断、セキュリティ境界、互換性、保留事項など、レビュアーが判断すべき点だけを具体的に書く。

Required sections:

- `概要`: what changed and the user/developer impact.
- `Stack構成`: Stack PRでの層、直下のbase、後続PR、層の境界。通常PRでは該当なしとする。
- `背景・原因`: why the change was needed; for non-fixes, state `該当なし` and explain the motivation instead.
- `やったこと`: short, concrete bullets.
- `変更結果`: observable behavior, files, generated output, screenshots, or command results. Do not add a video for a change that is adequately shown by text.
- `影響範囲`: 対象ユーザー・API/画面・互換性・設定やデータへの影響。影響がない場合も根拠とともに明記する。
- `やらないこと`: explicit out-of-scope items, especially `front/` for API-only work.
- `注意事項`: migration, startup, configuration, reviewer setup, or post-merge steps.
- `確認手順`: reproducible commands and manual steps, including prerequisites such as Docker/MySQL.
- `課題・特に見てほしい箇所`: unresolved concerns and focused review questions. Do not hide a known limitation.
- `備考`: related issues, designs, generated files, or other context.

Choose a title that states the result, not a vague activity. Keep the PR focused; split independent purposes instead of hiding them in one description. Stack PRでは、各層の本文だけで目的・境界・検証が分かるようにし、`Stack構成`と`備考`へ下位/上位PRを記す。Default to a Draft PR so unfinished work cannot be merged accidentally.

### 4. Commit, publish, and verify

Only perform the publish flow when the user asked to create the PR and the scope is unambiguous.

1. Require `gh --version` and `gh auth status` before committing or pushing. Stack PRでは`gh stack` extensionも確認する。利用できない場合は、公開を止めて正確なblockerを報告する。
2. Stage explicit paths only. Use the project `commit-message-ja` skill when it is available; use an English Conventional Commit prefix and a Japanese, intent-focused subject/body.
3. Run `git diff --cached --check`, then commit and record the commit SHA. Stack PRでは下位層から順に、各branchでこの手順と適切なテストを繰り返す。
4. 通常PRでは、現在のbranchを`git push -u origin <branch>`でpushする。Stack PRでは、branchを依存順に初期化・追加してから、`gh stack submit --auto --remote origin`を一度だけ実行する。`--auto`を省略しない。
5. Stackの各PRをDraftとして作成した後、PR番号・URL・base/head・Draft状態を確認する。自動生成された本文がテンプレートを満たさない場合は、各PRの実際の差分に基づく本文を用意し、`gh pr edit <number> --body-file <file>`などの非対話手段で設定する。
6. 通常PRはrepository default branchをbaseにする。Stack PRの2層目以降は直下のStack branchをbaseにする。レビュアーやlabelは、依頼またはrepository policyがない限り変更しない。
7. 作成後は、全PRのURL、title、Draft状態、base/head、変更ファイル、CI状態、conflict状態を確認する。StackではStack全体の順序と`needsRebase`も確認し、レビューまたはCI待ちの事項を報告する。

When the publish prerequisite fails, still leave the validated local skill and PR body ready, but do not imply that a commit, push, or PR was created.

## Output contract

Report:

- changed paths and scope decision;
- checks run with pass/fail/skipped status;
- PR title and body summary;
- commit, branch, push, and PR URL only when each operation actually succeeded;
- blockers and the precise next command or user action when publishing could not complete.
