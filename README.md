# gh-kusa-breaker

GitHubのコントリビューション（過去N週）をブロック強度に変換して遊べる、ターミナル用ブロック崩しです。`gh` 拡張として実行します。

## 前提
- `gh`（GitHub CLI）がインストール済み
- 事前にログイン済み（必須）:

```bash
gh auth login
```

## インストール（gh拡張）

```bash
gh extension install fchimpan/gh-kusa-breaker
```

（ローカルで試す場合）

```bash
gh extension install .
```

## 実行

```bash
gh kusa-breaker
```

オプション:

```bash
gh kusa-breaker --weeks 52
gh kusa-breaker --seed 123
```

## 操作
- ←/→ または `a`/`d`: パドル移動
- `q`: 終了

## 仕様メモ
- ログインユーザーのContribution CalendarをGitHub GraphQL APIから取得します（`go-gh` 経由）。
- コントリビューションが多いほどブロックが硬くなります（最大4段階）。
- 全ブロック破壊でクリアして終了します。


