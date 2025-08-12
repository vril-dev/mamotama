# mamotama

Coraza + CRS WAFプロジェクト

## 概要

このプロジェクトは、Coraza WAF と OWASP Core Rule Set (CRS) を組み合わせた
軽量かつ強力なアプリケーション防御システム「mamotama」です。

## ルールファイルについて

本リポジトリには、OWASP CRS のルールファイル（`rules/conf/*.conf`）や `crs-setup.conf` は含まれていません。

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

## 環境変数

以下のように `.env` ファイルで挙動を制御可能です

| 変数名 | 説明 | デフォルト |
| --- | --- | --- |
| `WAF_APP_URL` | バックエンドのURL（プロキシ先） | `（必須）` |
| `WAF_LOG_FILE` | WAFログ出力ファイルパス。未指定なら標準出力 | （空） |
| `WAF_BYPASS_FILE` | バイパス定義ファイル | `conf/waf.bypass` |
| `WAF_RULES_FILE` | 使用するルールファイル（カンマ区切りで複数指定可） | `rules/mamotama.conf` |
| `WAF_STRICT_OVERRIDE` | 特別ルールファイル読み込み失敗時の挙動を制御（trueで強制終了） | `false` |
| `NGX_CORAZA_UPSTREAM` | nginx用：Corazaの接続先を `server host:port;` 形式で指定（複数可） | `server coraza:9090;` |
| `NGX_BACKEND_RESPONSE_TIMEOUT` | nginx用：Corazaからの応答タイムアウト時間 | `60s` |
| `VITE_APP_BASE_PATH` | Reactダッシュボードのベースパス（例: `/mamotama-admin`） | `/mamotama-admin` |
| `VITE_API_BASE_PATH` | WAF APIのベースパス（例: `/mamotama-api`） | `/mamotama-api` |

## 管理ダッシュボード

`web/mamotama-admin/` 以下には、React + Vite による管理UIが含まれています。

### 主な画面と機能

| パス | 説明 |
| --- | --- |
| `/status` | WAFの動作状況、設定の確認 |
| `/logs` | WAFログの取得・表示 |
| `/rules` | 使用中のルールファイルの一覧表示 |
| `/bypass` | バイパス設定の閲覧・編集（waf.bypassを直接操作） |

### ライブラリ

* coraza 3.3.3
* openresty 1.27
* go 1.23
* React 19
* Vite 7
* Tailwind CSS
* react-router-dom
* ShadCN UI（TailwindベースUI）

### 起動方法

```bash
docker compose build coraza openresty
docker compose up -d coraza openresty
docker compose run --rm web npm install
docker compose run --rm web npm run dev
# or docker ... up web
```

環境変数 `.env` に `VITE_APP_BASE_PATH` および `VITE_API_BASE_PATH` を定義することで、ルートパスを変更できます。

## API管理エンドポイント（/mamotama-api）

### エンドポイント一覧

| メソッド | パス | 説明 |
| --- | --- | --- |
| GET | `/mamotama-api/status` | 現在のWAF設定状態を取得 |
| GET | `/mamotama-api/logs` | WAFログ（tail）を取得 |
| GET | `/mamotama-api/rules` | ルールファイル一覧を取得（複数対応） |
| GET | `/mamotama-admin/bypass` | バイパス設定ファイルの内容を取得 |
| POST | `/mamotama-admin/bypass` | バイパス設定ファイルを上書き保存 |

ログやルールが設定されていない場合は `500` で `{"error": "...説明..."}` を返します。

## WAFバイパス・特別ルール設定について

mamotamaでは、CorazaによるWAF検査を特定のリクエストに対して除外（バイパス）したり、特定のルールのみを適用する機能を備えています。

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

### UIからの編集

管理ダッシュボード `/bypass` 画面から、`waf.bypass` ファイルの内容を直接編集・保存できます。
この画面では、全体の設定内容をテキスト形式で表示・編集し、保存ボタンで即時適用できます。

### 優先順位

* 特別ルールが優先されます（同じパスにバイパス設定があっても無視）
* ルールファイルが存在しない場合

  * `WAF_STRICT_OVERRIDE=true` のときは即時強制終了（log.Fatalf）
  * `false` または未設定時はログ出力して通常ルールで処理継続

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

mamotama自体にはHTTPキャッシュ機能は搭載されていませんが、NginxやCloudflareなどの前段CDN/Reverse Proxyと併用することでキャッシュ制御が可能です。

推奨構成

* 静的ファイルやGET APIは `Cache-Control` や `Expires` をNginxで設定
* mamotama（Coraza）は主にWAF機能として特化運用

例

```nginx
location ~* \.(js|css|png|jpg)$ {
    expires 1d;
    add_header Cache-Control "public";
}
```

## 今後の予定

mamotama は従来のルールベース型WAF（Coraza + CRS）に加えて、将来的に AI による学習フィードバック型の構成を取り入れる予定です。

## 免責事項

本プロジェクトはセキュリティ学習・検証用途を主目的としており、本番運用環境で使用する際は十分な評価・チューニングを行ってください。
