# mamotama

Coraza + CRS WAFプロジェクト

## 概要

このプロジェクトは、Coraza WAF と OWASP Core Rule Set (CRS) を組み合わせた
軽量かつ強力なアプリケーション防御システム「mamotama」です。

## ルールファイルについて

本リポジトリには、OWASP CRS のルールファイル（`rules/conf/*.conf`）や `crs-setup.conf` は含まれていません。
これは以下の理由によるものです：

* CRSルールのライセンス上の懸念（Apache License 2.0に基づく再配布制限の回避）
* メンテナンスの簡素化
* 利用者が常に最新のCRSを使えるようにするため

### セットアップ手順

以下のコマンドでルールセットを取得・配置してください。

```bash
git clone https://github.com/coreruleset/coreruleset.git
cd coreruleset
cp crs-setup.conf.example ../coraza/rules/crs-setup.conf
cp rules/*.conf ../coraza/rules/conf/.
cp plugins/*.conf ../coraza/rules/conf/.
```

`rules/crs-setup.conf` は必要に応じて編集してください（`Paranoia Level` や `anomaly` スコアなど）。

## 環境変数 `.env` の説明

以下のように `.env` ファイルで挙動を制御可能です：

| 変数名                            | 説明                                                | デフォルト                 |
| ------------------------------ | ------------------------------------------------- | --------------------- |
| `WAF_APP_URL`                  | バックエンドのURL（プロキシ先）                                 | `（必須）`                |
| `WAF_LOG_FILE`                 | WAFログ出力ファイルパス。未指定なら標準出力                           | （空）                   |
| `WAF_BYPASS_FILE`              | バイパス定義ファイル                                        | `conf/waf.bypass`     |
| `WAF_RULES_FILE`               | 使用するルールファイル（カンマ区切りで複数指定可）                         | `rules/mamotama.conf` |
| `WAF_STRICT_OVERRIDE`          | 特別ルールファイル読み込み失敗時の挙動を制御（trueで強制終了）                 | `false`               |
| `NGX_CORAZA_UPSTREAM`          | nginx用：Corazaの接続先を `server host:port;` 形式で指定（複数可） | `server coraza:9090;` |
| `NGX_BACKEND_RESPONSE_TIMEOUT` | nginx用：Corazaからの応答タイムアウト時間                        | `60s`                 |
| `VITE_APP_BASE_PATH`           | Reactダッシュボードのベースパス（例: `/mamotama-admin`）          | `/mamotama-admin`     |
| `VITE_API_BASE_PATH`           | WAF APIのベースパス（例: `/mamotama-api`）                 | `/mamotama-api`       |

## 管理ダッシュボード

`web/mamotama-admin/` 以下には、React + Vite による管理UIが含まれています。

### 主な画面と機能

| パス        | 説明               |
| --------- | ---------------- |
| `/status` | WAFの動作状況、設定の確認   |
| `/logs`   | WAFログの取得・表示      |
| `/rules`  | 使用中のルールファイルの一覧表示 |

### 使用技術

* React 19
* Vite 7
* Tailwind CSS
* react-router-dom
* ShadCN UI（TailwindベースUI）

### 起動方法

```bash
cd web/mamotama-admin
npm install
npm run dev
```

環境変数 `.env` に `VITE_APP_BASE_PATH` および `VITE_API_BASE_PATH` を定義することで、ルートパスを変更できます。

## API管理エンドポイント（/mamotama-api）

バージョン `v0.3.0` 以降、Coraza WAF バックエンドは Gin を導入し、API形式で情報を取得可能です。

### エンドポイント一覧

| メソッド | パス                     | 説明                 |
| ---- | ---------------------- | ------------------ |
| GET  | `/mamotama-api/status` | 現在のWAF設定状態を取得      |
| GET  | `/mamotama-api/logs`   | WAFログ（tail）を取得     |
| GET  | `/mamotama-api/rules`  | ルールファイル一覧を取得（複数対応） |

ログやルールが設定されていない場合は `500` で `{"error": "...説明..."}` を返します。

## WAFバイパス・特別ルール設定について

mamotamaでは、CorazaによるWAF検査を\*\*特定のリクエストに対して除外（バイパス）\*\*したり、\*\*特定のルールのみを適用する（特別ルール）\*\*機能を備えています。

### バイパスファイルの指定

環境変数 `WAF_BYPASS_FILE` で除外・特別ルール定義ファイルを指定します。デフォルトは `conf/waf.bypass` です。

### ファイル記述形式

```text
# 通常のバイパス指定
/about/
/about/user.php

# 特別ルール適用（WAFバイパスせず、指定ルールを使用）
/about/admin.php rules/admin-rule.conf

# コメント（先頭 #）
#/should/be/ignored.php rules/test.conf
```

### 優先順位

* **特別ルールが優先**されます（同じパスにバイパス設定があっても無視）
* ルールファイルが存在しない場合：

  * `WAF_STRICT_OVERRIDE=true` のときは **即時強制終了（log.Fatalf）**
  * `false` または未設定時は **ログ出力して通常ルールで処理継続**

### 例

```text
/about/                    # /about/ 以下すべてバイパス
/about/admin.php rules/special.conf  # admin.php だけは WAF で特別ルール適用
```

### 注意

* ルール記述はファイル上で上から順に評価されます
* `extraRuleFile` を指定した行が優先されます
* コメント行（`#`で始まる）は無視されます

## キャッシュ制御について

mamotama自体にはHTTPキャッシュ機能は搭載されていませんが、**NginxやCloudflareなどの前段CDN/Reverse Proxyと併用することでキャッシュ制御が可能**です。

推奨構成：

* 静的ファイルやGET APIは `Cache-Control` や `Expires` をNginxで設定
* mamotama（Coraza）は主にWAF機能として特化運用

例：

```nginx
location ~* \.(js|css|png|jpg)$ {
    expires 1d;
    add_header Cache-Control "public";
}
```

## 利用と意義について

Cloudflareなどの無料WAF/CDNが存在する現在、mamotamaは以下のような要件に特化した価値を提供します：

* 独自ルールの完全な自由度
* 誤検知への柔軟な除外対応（`waf.bypass`）
* ログ出力や監視のフルコントロール
* 内部向け（管理画面/API）や閉域網でも利用可能

## 今後の実装予定：AI活用によるゼロデイ攻撃対応構想

mamotama は従来のルールベース型WAF（Coraza + CRS）に加えて、将来的に AI による学習フィードバック型の構成を取り入れる予定です。これにより、未知の攻撃パターン（いわゆるゼロデイ攻撃）への対応力を高めていきます。

### 概要

以下のような構成を計画中です：

| 機能         | 内容                                                 |
| ---------- | -------------------------------------------------- |
| リクエスト判定フロー | 通常は既存WAFルールで即時処理し、異常または未マッチパターンのみをAIへ送出            |
| ログ自動収集     | `logs` API 経由で未知リクエストを収集、AI学習候補として蓄積               |
| オフライン学習    | 収集したリクエストログから自動的にルール候補を生成（XGBoost/TF-IDF等）         |
| ルール候補提示    | フロントUI上で「新規候補ルール」として提示、採用可否を管理者が選択可能に              |
| ルール即時適用    | 採用されたルールは `rules/generated.conf` 等に追記され、自動で次回判定に反映 |

### 実現する価値

* 攻撃検知の進化：未知の攻撃を“次回以降検知可能”に
* 自己進化型WAF：検知力を継続強化できる構成
* ブラックボックスでなく、“説明可能なAI WAF”

### 備考

* 本構成はあくまで自律強化補助であり、AIの判断は最終的に人のレビューを前提とする
* 外部TI連携（AbuseIPDB等）との統合も視野に入れている

この実装が完了すれば、mamotamaは「学習型WAF」へ進化し、柔軟性と可視性を備えた構成となる予定です。

## バージョン履歴

### v0.3.0

* GinベースのWAF API追加（/status, /logs, /rules）
* React + Viteによる管理ダッシュボードを追加
* ルールファイル複数対応（カンマ区切り指定）
* `/mamotama-api` にルーティングを統合し、API/フロントを明確に分離
* Nginx経由でのAPI/フロント統合構成を正式サポート

## 免責事項

本プロジェクトはセキュリティ学習・検証用途を主目的としており、本番運用環境で使用する際は十分な評価・チューニングを行ってください。
