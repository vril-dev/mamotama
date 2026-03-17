# mamotama

Coraza + CRS WAFプロジェクト

[English](README.md) | [日本語](README.ja.md)

![管理画面トップ](docs/images/admin-dashboard-overview.png)

## 概要

このプロジェクトは、Coraza WAF と OWASP Core Rule Set (CRS) を組み合わせた
軽量かつ強力なアプリケーション防御システム「mamotama」です。

---

## ルールファイルについて

本リポジトリには、ライセンス順守のため OWASP CRS 本体は同梱していません。  
代わりに、初期状態で動作可能な最小ベースルール `data/rules/mamotama.conf` を同梱しています。

### セットアップ手順

以下のコマンドで CRS を取得・配置してください（デフォルト: `v4.23.0`）。

```bash
./scripts/install_crs.sh
```

バージョン指定例:

```bash
./scripts/install_crs.sh v4.23.0
```

`data/rules/crs/crs-setup.conf` は必要に応じて編集してください（`Paranoia Level` や `anomaly` スコアなど）。

---

## 環境変数

`.env` ファイルで挙動を制御可能です。

### Nginx（openresty 側）

| 変数名 | 例 | 説明 |
| --- | --- | --- |
| `NGX_CORAZA_UPSTREAM` | `server coraza:9090;` | Coraza（Goサーバ）の upstream 定義。`server host:port;` を複数行で並べれば簡易ロードバランス可。 |
| `NGX_BACKEND_RESPONSE_TIMEOUT` | `60s` | 上流（Coraza）からの応答タイムアウト。`proxy_read_timeout` に反映。 |
| `NGX_CORAZA_ADMIN_URL` | `/mamotama-admin/` | 管理UIの公開パス。末尾スラッシュ必須。このパスに来たリクエストをフロント（`web:5173`）へプロキシ。 |
| `NGX_CORAZA_API_BASEPATH` | `/mamotama-api/` | 管理APIのベースパス。末尾スラッシュ推奨。このパス配下は nginx 側で常に非キャッシュ扱い。 |

### WAF / Go（Coraza ラッパー）

| 変数名 | 例 | 説明 |
| --- | --- | --- |
| `WAF_APP_URL` | `http://host.docker.internal:3000` | 透過先アプリの URL（ALB/ECS 等の本番では適宜変更）。 |
| `WAF_LOG_FILE` | (空) | WAFログの出力先。未設定なら標準出力。 |
| `WAF_BYPASS_FILE` | `conf/waf.bypass` | バイパス/特別ルール定義ファイルのパス。 |
| `WAF_BOT_DEFENSE_FILE` | `conf/bot-defense.conf` | Bot defense challenge 設定ファイル（JSON）。管理画面から編集可能。 |
| `WAF_SEMANTIC_FILE` | `conf/semantic.conf` | Semanticヒューリスティック設定ファイル（JSON）。管理画面から編集可能。 |
| `WAF_COUNTRY_BLOCK_FILE` | `conf/country-block.conf` | 国別ブロック定義ファイル（1行1国コード、例: `JP`, `US`, `UNKNOWN`）。 |
| `WAF_RATE_LIMIT_FILE` | `conf/rate-limit.conf` | レート制限定義ファイル（JSON）。管理画面から編集可能。 |
| `WAF_RULES_FILE` | `rules/mamotama.conf` | 使用するルールファイル（カンマ区切りで複数指定も可）。 |
| `WAF_CRS_ENABLE` | `true` | CRSを読み込むかどうか。`false` ならベースルールのみ。 |
| `WAF_CRS_SETUP_FILE` | `rules/crs/crs-setup.conf` | CRSセットアップ設定ファイル。 |
| `WAF_CRS_RULES_DIR` | `rules/crs/rules` | CRS本体ルール（`*.conf`）のディレクトリ。 |
| `WAF_CRS_DISABLED_FILE` | `conf/crs-disabled.conf` | CRS本体の無効化ファイル一覧。1行1ファイル名で指定。 |
| `WAF_STRICT_OVERRIDE` | `false` | 特別ルール読み込み失敗時の挙動。`true`で即終了、`false`で警告のみ継続。 |
| `WAF_API_BASEPATH` | `/mamotama-api` | 管理APIのベースパス（Go側のルーティング基準）。 |
| `WAF_API_KEY_PRIMARY` | `…` | 管理API用の主キー（`X-API-Key`）。 |
| `WAF_API_KEY_SECONDARY` | (空) | 予備キー（ローテーション時の切替用。未使用なら空でOK）。 |
| `WAF_API_AUTH_DISABLE` | (空) | 認証無効化フラグ。運用では空（false相当）推奨。テストで無効化したいときのみ truthy 値。 |
| `WAF_API_CORS_ALLOWED_ORIGINS` | `https://admin.example.com,http://localhost:5173` | CORSを許可する Origin 一覧（カンマ区切り）。未設定なら CORS 無効（同一オリジンのみ）。 |
| `WAF_ALLOW_INSECURE_DEFAULTS` | (空) | 弱いAPIキーや認証無効化を許可する開発用フラグ。本番では設定しない。 |

### 管理UI（React / Vite）

| 変数名 | 例 | 説明 |
| --- | --- | --- |
| `VITE_CORAZA_API_BASE` | `http://localhost/mamotama-api` | ブラウザから叩く API のフル/相対ベース。リバースプロキシの都合に合わせて指定。 |
| `VITE_APP_BASE_PATH` | `/mamotama-admin` | 管理UIのルートパス（`react-router` の basename）。 |
| `VITE_API_KEY` | `…` | 管理UIが API へ付与する `X-API-Key`。通常は `WAF_API_KEY_PRIMARY` と同値。 |

起動時に `WAF_API_KEY_PRIMARY` が短すぎる/既知の弱い値の場合、Corazaプロセスは安全側で起動失敗します。  
ローカル検証だけ一時的に緩和したい場合は `WAF_ALLOW_INSECURE_DEFAULTS=1` を利用してください。

---

## 管理ダッシュボード

`web/mamotama-admin/` 以下には、React + Vite による管理UIが含まれています。

![管理画面 Dashboard](docs/images/admin-dashboard-overview.png)

### 主な画面と機能

| パス | 説明 |
| --- | --- |
| `/status` | WAFの動作状況、設定の確認 |
| `/logs` | WAFログの取得・表示 |
| `/rules` | 使用中のルールファイルの一覧表示 |
| `/rule-sets` | CRS本体ルール（`rules/crs/rules/*.conf`）の有効/無効切替 |
| `/bypass` | バイパス設定の閲覧・編集（waf.bypassを直接操作） |
| `/country-block` | 国別ブロック設定の閲覧・編集（country-block.conf を直接操作） |
| `/rate-limit` | レート制限設定の閲覧・編集（rate-limit.conf を直接操作） |
| `/bot-defense` | Bot defense設定の閲覧・編集（bot-defense.conf を直接操作） |
| `/semantic` | Semantic Security設定の閲覧・編集（semantic.conf を直接操作） |
| `/cache-rules` | Cache Rules の可視化・編集（cache.conf の表編集／Raw編集、Validate/Save対応） |

### 画面キャプチャ

#### Dashboard
![Dashboard](docs/images/admin-dashboard-overview.png)

#### Rules Editor
![Rules Editor](docs/images/admin-rules-editor.png)

#### Rule Sets
![Rule Sets](docs/images/admin-rule-sets.png)

#### Bypass Rules
![Bypass Rules](docs/images/admin-bypass-rules.png)

#### Country Block
![Country Block](docs/images/admin-country-block.png)

#### Rate Limit
![Rate Limit](docs/images/admin-rate-limit.png)

#### Cache Rules
![Cache Rules](docs/images/admin-cache-rules.png)

#### Logs
![Logs](docs/images/admin-logs.png)

### ライブラリ

* coraza 3.3.3
* openresty 1.27
* go 1.25.7
* React 19
* Vite 7
* Tailwind CSS
* react-router-dom
* ShadCN UI（TailwindベースUI）

### 起動方法

```bash
./scripts/install_crs.sh
docker compose build coraza openresty
docker compose up web
docker compose up -d coraza openresty
```

環境変数 `.env` に `VITE_APP_BASE_PATH` および `VITE_CORAZA_API_BASE` を定義することで、ルートパスを変更できます。

### WAF回帰テスト（GoTestWAF）

ローカルで回帰テストを実行:

```bash
./scripts/run_gotestwaf.sh
```

前提条件:

- Docker と Docker Compose が利用可能であること
- スクリプトが `coraza` と `openresty` を自動で build/up すること
- 既定のホスト公開ポートは `HOST_CORAZA_PORT=19090` と `HOST_OPENRESTY_PORT=18080`
- 初回実行時は GoTestWAF イメージ取得のため時間がかかる場合があること

デフォルトの合否基準は `MIN_BLOCKED_RATIO=70` です。追加基準は任意で指定できます:

```bash
MIN_TRUE_NEGATIVE_PASSED_RATIO=95 MAX_FALSE_POSITIVE_RATIO=5 MAX_BYPASS_RATIO=30 ./scripts/run_gotestwaf.sh
```

レポート出力先は `data/logs/gotestwaf/` です:

- JSONフルレポート: `gotestwaf-report.json`
- Markdownサマリ: `gotestwaf-report-summary.md`
- Key-Valueサマリ: `gotestwaf-report-summary.txt`

---

## API管理エンドポイント（/mamotama-api）

### エンドポイント一覧

| メソッド | パス | 説明 |
| --- | --- | --- |
| GET | `/mamotama-api/status` | 現在のWAF設定状態を取得 |
| GET | `/mamotama-api/logs/read` | WAFログ（tail）を取得（`country` クエリで国別フィルタ可） |
| GET | `/mamotama-api/logs/download` | 3種類のログファイル（`waf` / `accerr` / `intr`）をZIPでまとめてダウンロード |
| GET | `/mamotama-api/rules` | ルールファイル一覧を取得（複数対応） |
| POST | `/mamotama-api/rules:validate` | 指定ルールファイルの構文検証（保存なし） |
| PUT | `/mamotama-api/rules` | 指定ルールファイルを保存し、WAFベースルールをホットリロード（`If-Match`対応） |
| GET | `/mamotama-api/crs-rule-sets` | CRS本体ルール一覧と有効/無効状態を取得 |
| POST | `/mamotama-api/crs-rule-sets:validate` | CRS本体ルール選択の検証（保存なし） |
| PUT | `/mamotama-api/crs-rule-sets` | CRS本体ルール選択を保存し、ホットリロード（`If-Match`対応） |
| GET | `/mamotama-api/bypass-rules` | バイパス設定ファイルの内容を取得 |
| POST | `/mamotama-api/bypass-rules:validate` | 送信内容の構文・検証のみ（保存なし） |
| PUT | `/mamotama-api/bypass-rules` | バイパス設定ファイルを上書き保存（`If-Match` に `ETag` を指定して楽観ロック） |
| GET  | `/mamotama-api/country-block-rules` | 国別ブロック設定ファイルの内容を取得 |
| POST | `/mamotama-api/country-block-rules:validate` | 国別ブロック設定の構文検証のみ（保存なし） |
| PUT  | `/mamotama-api/country-block-rules` | 国別ブロック設定ファイルを保存（`If-Match` に `ETag` を指定して楽観ロック） |
| GET  | `/mamotama-api/rate-limit-rules` | レート制限設定ファイルの内容を取得 |
| POST | `/mamotama-api/rate-limit-rules:validate` | レート制限設定の構文検証のみ（保存なし） |
| PUT  | `/mamotama-api/rate-limit-rules` | レート制限設定ファイルを保存（`If-Match` に `ETag` を指定して楽観ロック） |
| GET  | `/mamotama-api/bot-defense-rules` | Bot defense設定ファイルの内容を取得 |
| POST | `/mamotama-api/bot-defense-rules:validate` | Bot defense設定の構文検証のみ（保存なし） |
| PUT  | `/mamotama-api/bot-defense-rules` | Bot defense設定ファイルを保存（`If-Match` に `ETag` を指定して楽観ロック） |
| GET  | `/mamotama-api/semantic-rules` | Semantic設定と実行統計を取得 |
| POST | `/mamotama-api/semantic-rules:validate` | Semantic設定の構文検証のみ（保存なし） |
| PUT  | `/mamotama-api/semantic-rules` | Semantic設定ファイルを保存（`If-Match` に `ETag` を指定して楽観ロック） |
| GET  | `/mamotama-api/cache-rules` | cache.conf の現在内容（Raw + 構造化）と `ETag` を返す |
| POST | `/mamotama-api/cache-rules:validate` | 送信内容の構文・検証のみ（保存なし） |
| PUT | `/mamotama-api/cache-rules` | cache.conf を保存（`If-Match` に `ETag` を指定して楽観ロック） |


ログやルールが設定されていない場合は `500` で `{"error": "...説明..."}` を返します。

---

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

### 国別ブロック設定

管理ダッシュボード `/country-block` から、`WAF_COUNTRY_BLOCK_FILE`（既定: `conf/country-block.conf`）を編集できます。  
1行に1つの国コードを記述します（例: `JP`, `US`, `UNKNOWN`）。  
該当する国コードのアクセスは WAF 前段で `403` になります。

### レート制限設定

管理ダッシュボード `/rate-limit` から、`WAF_RATE_LIMIT_FILE`（既定: `conf/rate-limit.conf`）を編集できます。  
設定は JSON 形式で、`default_policy` と `rules` を管理します。  
超過時は `action.status`（通常 `429`）を返し、`Retry-After` ヘッダを付与します。

#### JSONパラメータ早見表（何を変えるとどうなるか）

| パラメータ | 例 | 影響 |
| --- | --- | --- |
| `enabled` | `true` / `false` | レート制限全体の有効/無効。`false` なら全リクエストを素通し。 |
| `allowlist_ips` | `["127.0.0.1/32", "10.0.0.5"]` | 一致IPは常に制限対象外。CIDRと単体IPの両方を指定可。 |
| `allowlist_countries` | `["JP", "US"]` | 一致国コードは常に制限対象外。 |
| `default_policy.enabled` | `true` | デフォルトポリシー自体の有効/無効。 |
| `default_policy.limit` | `120` | ウィンドウ期間内の基本許可回数。 |
| `default_policy.burst` | `20` | `limit` に上乗せする瞬間許容量。実効上限は `limit + burst`。 |
| `default_policy.window_seconds` | `60` | カウント窓の秒数。短いほど厳密、長いほど緩やか。 |
| `default_policy.key_by` | `"ip"` | 集計キー。`ip` / `country` / `ip_country`。 |
| `default_policy.action.status` | `429` | 超過時のHTTPステータス。`4xx/5xx`のみ。 |
| `default_policy.action.retry_after_seconds` | `60` | `Retry-After` ヘッダ秒数。`0` なら次ウィンドウまでの残秒を自動計算。 |
| `rules[]` | 下記参照 | 条件一致時に `default_policy` より優先して適用。先頭から順に評価。 |
| `rules[].match_type` | `"prefix"` | ルールの一致方式。`exact` / `prefix` / `regex`。 |
| `rules[].match_value` | `"/login"` | 一致対象。`match_type` に応じて完全一致/前方一致/正規表現。 |
| `rules[].methods` | `["POST"]` | 対象メソッド限定。空なら全メソッド対象。 |
| `rules[].policy.*` |  | ルール一致時に使う制限値（`default_policy` と同じ意味）。 |

#### 運用でよくやる調整

- 全体を一時停止したい: `enabled=false`
- 短時間スパイクに強くしたい: `burst` を増やす
- ログインだけ厳しくしたい: `rules` に `match_type=prefix`, `match_value=/login`, `methods=["POST"]` を追加
- 同一IP内で国別に分けたい: `key_by="ip_country"`
- 特定拠点を除外したい: `allowlist_ips` または `allowlist_countries` に追加

### Bot Defense 設定

管理ダッシュボード `/bot-defense` から、`WAF_BOT_DEFENSE_FILE`（既定: `conf/bot-defense.conf`）を編集できます。  
有効時は、対象パスの GET リクエストに対して（`mode` に応じて）challenge レスポンスを返し、通過後に通常処理へ進みます。

#### JSONパラメータ早見表

| パラメータ | 例 | 影響 |
| --- | --- | --- |
| `enabled` | `true` / `false` | Bot challenge の全体ON/OFF。 |
| `mode` | `"suspicious"` | `suspicious` は UA 条件一致時のみ、`always` は一致パスを常に challenge。 |
| `path_prefixes` | `["/", "/login"]` | challenge 対象のパス前方一致。 |
| `exempt_cidrs` | `["127.0.0.1/32"]` | challenge 除外する送信元 IP/CIDR。 |
| `suspicious_user_agents` | `["curl", "wget"]` | `suspicious` モードで使う UA 部分一致。 |
| `challenge_cookie_name` | `"__mamotama_bot_ok"` | challenge 通過に使う Cookie 名。 |
| `challenge_secret` | `"long-random-secret"` | challenge トークン署名シークレット（空ならプロセス起動ごとに一時生成）。 |
| `challenge_ttl_seconds` | `86400` | challenge トークン有効期限（秒）。 |
| `challenge_status_code` | `429` | challenge 応答時の HTTP ステータス（`4xx/5xx`）。 |

### Semantic Security 設定

管理ダッシュボード `/semantic` から、`WAF_SEMANTIC_FILE`（既定: `conf/semantic.conf`）を編集できます。  
これは機械学習ではなくルールベースのヒューリスティック検知で、`off | log_only | challenge | block` の段階制御に対応します。

#### JSONパラメータ早見表

| パラメータ | 例 | 影響 |
| --- | --- | --- |
| `enabled` | `true` / `false` | semantic スコアリング全体の有効/無効。 |
| `mode` | `"challenge"` | 実行モード。`off` / `log_only` / `challenge` / `block`。 |
| `exempt_path_prefixes` | `["/healthz"]` | 一致パスは semantic 検査をスキップ。 |
| `log_threshold` | `4` | anomaly ログを出す最小スコア。 |
| `challenge_threshold` | `7` | `challenge` モードで challenge 応答にする最小スコア。 |
| `block_threshold` | `9` | `block` モードで `403` にする最小スコア。 |
| `max_inspect_body` | `16384` | semantic が検査するリクエストボディ最大バイト数。 |

### ルールファイル編集（複数対応）

管理ダッシュボード `/rules` では、アクティブなベースルールセットを選択して編集できます（`WAF_RULES_FILE` と、CRS有効時は `crs-setup.conf` + 有効化されている `WAF_CRS_RULES_DIR` の `*.conf`）。  
保存時はサーバ側で構文検証した後に反映され、Coraza のベースルールセットをホットリロードします。  
リロード失敗時は自動でロールバックされます。

### CRSルールセット切替

管理ダッシュボード `/rule-sets` では、`rules/crs/rules/*.conf` の各ファイルを有効/無効で切り替えられます。  
状態は `WAF_CRS_DISABLED_FILE` に保存され、保存時にWAFをホットリロードします。

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

---

## ログの確認

本システムのログは API 経由で取得できます。

```bash
curl -s -H "X-API-Key: <your-api-key>" \
     "http://<host>/mamotama-api/logs/read?src=waf&tail=100&country=JP" | jq .
```

* src: ログ種別 (waf, accerr, intr)
* tail: 取得件数
* country: 国コード（例: `JP`, `US`, `UNKNOWN`。未指定または`ALL`で全件）
  * Cloudflare配下では `CF-IPCountry` ヘッダを利用します。未取得時は `UNKNOWN` になります。

API キーは .env で設定した API_KEY を使用してください。
実運用環境ではアクセス制限や認証を必ず設定してください。

## キャッシュ機能

キャッシュ対象のパスやTTLを動的に設定できる機能を追加しました。

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
- `Set-Cookie` を含む上流レスポンスは保存されません（共有キャッシュ誤配信防止）

### 確認方法

- レスポンスヘッダに以下が含まれているか確認
  - `X-Mamotama-Cacheable: 1`
  - `X-Accel-Expires: <秒数>`
- nginx の `X-Cache-Status` ヘッダでキャッシュヒット状況を確認可能（MISS/HIT/BYPASS 等）

---

## 管理画面のアクセス制限について

本プロジェクトにはデフォルトでアクセス制限機能は含まれていません。  
管理画面（NGX_CORAZA_ADMIN_URL で公開されるパス）を利用する場合は、必ず Basic 認証や IP 制限などのアクセス制御を設定してください。

---

## 品質ゲート（CI）

GitHub Actions の `ci` ワークフローで以下を検証します。

- `go test ./...`（`coraza/src`）
- `docker compose config` の妥当性確認
- `./scripts/run_gotestwaf.sh`（`waf-test` ジョブ、`MIN_BLOCKED_RATIO=70`）

運用では、以下をブランチ保護の Required Checks に設定してください。

- `ci / go-test`
- `ci / compose-validate`
- `ci / waf-test`

---

## 誤検知チューニング運用

誤検知の削減手順は以下を参照してください。

- `docs/operations/waf-tuning.md`

---

## 免責事項

本プロジェクトはセキュリティ学習・検証用途を主目的としており、本番運用環境で使用する際は十分な評価・チューニングを行ってください。
