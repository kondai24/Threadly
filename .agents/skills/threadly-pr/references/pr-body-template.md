# Threadly PR body template

Use this template after inspecting the actual diff. Replace every placeholder; remove a section only when it is genuinely not applicable and write `該当なし` with a reason when that is clearer.

```markdown
## 概要

<!-- 何を変更し、誰にどんな影響があるか。結論を先に書く。 -->

## 背景・原因

<!-- なぜ変更したか。バグ修正でなければ「該当なし」ではなく、変更の動機を書く。 -->

## やったこと

- <!-- 変更1 -->
- <!-- 変更2 -->

## 変更結果

<!-- 観測できる結果、レスポンス、画面、生成物、テスト結果など。未確認の期待値は書かない。 -->

## やらないこと

- <!-- このPRの対象外。例: APIのみのためfront/は変更しない -->

## 注意事項

- <!-- マージ後のmigration、環境変数、起動条件、手動作業など。なければ「なし」。 -->

## 確認手順

### 自動確認

- [ ] `<!-- command -->`
- [ ] `<!-- command -->`

### 手動確認

1. <!-- 前提条件 -->
2. <!-- 操作 -->
3. <!-- 期待結果 -->

## 課題・特に見てほしい箇所

- <!-- 未解決の懸念、判断してほしい設計、境界条件。なければ「なし」。 -->

## 備考

- <!-- Issue、設計ノート、生成コマンド、関連PRなど。なければ「なし」。 -->
```

## Writing rules

- Keep the summary and impact understandable without reading the diff first.
- Distinguish facts, interpretations, and open questions.
- Include exact commands and their actual result; label environment-dependent checks as skipped or blocked.
- Prefer a short screenshot for UI changes. Use a short video only when interaction or timing cannot be understood from text or images; avoid long GIFs.
- Link repository issues and public design documents, but never publish private Obsidian paths or personal notes.
