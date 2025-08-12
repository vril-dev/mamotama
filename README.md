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
docker compose up web
docker compose up -d coraza openresty
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

## キャッシュ機能（0.4.1以降）

mamotama 0.4.1 から、キャッシュ対象のパスやTTLを動的に設定できる機能を追加しました。

### 設定ファイル
キャッシュ設定は `/data/conf/cache.conf` に記述します。  
設定変更はホットリロードに対応しており、ファイル保存後すぐに反映されます。

#### 記述例

```bash
# 静的アセット（CSS/JS/画像）を10分キャッシュ
ALLOW prefix=/_next/static/chunks/ methods=GET,HEAD ttl=600 vary=Accept-Encoding

# 特定HTMLページ群を5分キャッシュ（正規表現）
ALLOW regex=^/about/.*.html$ methods=GET ttl=300

# API全域禁止（安全側）
DENY prefix=/mamotama-api/

# 認証ユーザーのプロフィールはキャッシュ禁止（正規表現）
DENY regex=^/users/[0-9]+/profile

# その他はデフォルトでキャッシュ禁止
```

- ALLOW: キャッシュ許可（TTLは秒単位、Varyは任意）
- DENY: キャッシュ対象外
- メソッドは `GET` または `HEAD` を推奨（POST等はキャッシュされません）

フィールド説明
- prefix: 指定パスで始まる場合にマッチ
- regex: 正規表現でマッチ（`^`や`$`を使って指定可能）
- methods: 対象HTTPメソッド（カンマ区切り）
- ttl: キャッシュ時間（秒）
- vary: nginxに渡すVaryヘッダ値（カンマ区切り）

### 動作概要

- Go側でルールに一致したレスポンスに `X-Mamotama-Cacheable` と `X-Accel-Expires` を付与
- nginx がこれらのヘッダを元にキャッシュを管理
- 認証付きリクエスト、Cookieあり、APIパスはデフォルトでキャッシュされません

### 確認方法

- レスポンスヘッダに以下が含まれているか確認
  - `X-Mamotama-Cacheable: 1`
  - `X-Accel-Expires: <秒数>`
- nginx の `X-Cache-Status` ヘッダでキャッシュヒット状況を確認可能（MISS/HIT/BYPASS 等）

## 今後の予定

mamotama は従来のルールベース型WAF（Coraza + CRS）に加えて、将来的に AI による学習フィードバック型の構成を取り入れる予定です。

## 免責事項

本プロジェクトはセキュリティ学習・検証用途を主目的としており、本番運用環境で使用する際は十分な評価・チューニングを行ってください。
