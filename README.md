# imagedupfinder

画像の重複・類似検出を行う CLI ツール。リサイズや圧縮された画像も検出可能。

## 特徴

- **Perceptual Hash (pHash)** を使用し、リサイズ・圧縮後の画像も類似検出
- **Exact モード** でバイト単位の完全一致検出（SHA256）
- **自動スコアリング** で最高品質の画像を自動選択
- **並列処理** で大量画像を高速スキャン
- **SQLite** でインデックスを永続化（差分スキャン対応）

## インストール

```bash
go install github.com/yourname/imagedupfinder@latest
```

または、ソースからビルド:

```bash
git clone https://github.com/yourname/imagedupfinder.git
cd imagedupfinder
go build -o imagedupfinder .
```

## 使い方

### 1. スキャン

フォルダを再帰的にスキャンして重複を検出:

```bash
imagedupfinder scan ~/Pictures
```

完全一致のみを検出（SHA256 ハッシュ比較）:

```bash
imagedupfinder scan ~/Pictures --exact
```

### 2. 重複一覧

検出された重複グループを表示:

```bash
imagedupfinder list
```

出力例:

```
Found 3 duplicate groups (7 duplicates, 15.2 MB reclaimable)

Group #1 (3 images)
------------------------------------------------------------
  ✓ photo_original.png      3840x2160  PNG     8.2 MB  Score: 9953280
  ✗ photo_resized.jpg       1920x1080  JPEG    1.2 MB  Score: 2073600
  ✗ photo_thumbnail.jpg      640x360   JPEG    120 KB  Score: 230400
```

- `✓` = 残す画像（最高スコア）
- `✗` = 削除対象

### 3. クリーンアップ

削除対象をプレビュー:

```bash
imagedupfinder clean --dry-run
```

実行（確認プロンプトあり）:

```bash
imagedupfinder clean
```

削除の代わりに移動:

```bash
imagedupfinder clean --move-to=./duplicates
```

確認をスキップ:

```bash
imagedupfinder clean --yes
```

特定のグループのみ処理:

```bash
imagedupfinder clean --group=1           # グループ1のみ
imagedupfinder clean -g 1 -g 3           # グループ1と3
imagedupfinder clean --group=1,3,5       # カンマ区切りも可
```

## スコアリング

最高品質の画像を自動選択するスコアリング:

```
スコア = 解像度 (width × height) × フォーマット係数 × メタデータ係数
```

### フォーマット係数

| フォーマット | 係数 | 理由 |
|-------------|------|------|
| PNG / TIFF / BMP | 1.2 | 無圧縮・可逆圧縮 |
| WebP | 1.1 | 高効率圧縮 |
| JPEG | 1.0 | 非可逆圧縮 |
| GIF | 0.9 | 色数制限 |

### メタデータ係数

| 条件 | 係数 | 理由 |
|------|------|------|
| EXIF あり | 1.1 | 撮影情報を保持 |
| EXIF なし | 1.0 | - |

SNS からダウンロードした画像は EXIF が削除されていることが多いため、オリジナルの画像（EXIF あり）を優先的に残します。

### 同スコアの場合

1. ファイルサイズが大きい（より多くの情報を含む）
2. 更新日時が新しい
3. パスのアルファベット順

## オプション

| フラグ | デフォルト | 説明 |
|--------|-----------|------|
| `--exact` | false | 完全一致モード（SHA256 ハッシュで比較） |
| `--threshold` | 10 | ハミング距離の閾値（0-64、小さいほど厳密） |
| `--workers` | 8 | 並列ワーカー数 |
| `--db` | `~/.imagedupfinder/images.db` | SQLite データベースパス |

### モードの選択

| モード | オプション | 用途 |
|--------|-----------|------|
| Perceptual | (デフォルト) | リサイズ・圧縮された画像も検出 |
| Exact | `--exact` | バイト単位で完全一致する画像のみ検出 |

### 閾値の目安（Perceptual モード）

| 値 | 用途 |
|----|------|
| 0 | pHash が完全一致する画像のみ |
| 1-5 | ほぼ同一の画像のみ検出 |
| 5-10 | 軽微な編集・圧縮も検出（推奨） |
| 10-15 | 類似画像も検出（誤検出増加の可能性） |

## 対応フォーマット

- JPEG (.jpg, .jpeg)
- PNG (.png)
- GIF (.gif)
- WebP (.webp)
- BMP (.bmp)
- TIFF (.tiff, .tif)

## アーキテクチャ

```
imagedupfinder/
├── main.go
├── cmd/
│   ├── root.go      # CLI エントリポイント
│   ├── scan.go      # scan コマンド
│   ├── list.go      # list コマンド
│   └── clean.go     # clean コマンド
└── internal/
    ├── models.go    # データ構造
    ├── hasher.go    # pHash 計算
    ├── scanner.go   # 並列スキャン
    ├── grouper.go   # 重複グループ検出 (Union-Find)
    └── storage.go   # SQLite 永続化
```

## ライセンス

MIT License
