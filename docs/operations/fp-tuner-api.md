# FP Tuner API Contract (v1)

This document defines the current API contract for mock-based FP tuning flow.

## Endpoints

- `POST /mamotama-api/fp-tuner/propose`
- `POST /mamotama-api/fp-tuner/apply`

## 1) Propose

### Request

```json
{
  "target_path": "rules/mamotama.conf",
  "event": {
    "event_id": "manual-test-001",
    "method": "GET",
    "path": "/search",
    "rule_id": 100004,
    "status": 403,
    "matched_variable": "ARGS:q",
    "matched_value": "select * from users"
  }
}
```

Notes:
- `event` is optional. If omitted, server tries latest `waf_block` event from `waf-events.ndjson`.
- Unknown fields are rejected.

### Response

```json
{
  "ok": true,
  "contract_version": "fp_tuner.v1",
  "mode": "mock",
  "source": "request",
  "input": {
    "event_id": "manual-test-001",
    "method": "GET",
    "path": "/search",
    "rule_id": 100004,
    "status": 403,
    "matched_variable": "ARGS:q",
    "matched_value": "select * from users"
  },
  "proposal": {
    "id": "fp-mock-001",
    "title": "Scoped false-positive tuning suggestion",
    "summary": "Mock provider response for fp-tuner contract testing.",
    "reason": "Fixture-based response to test send/receive/apply flow without external LLM API.",
    "confidence": 0.84,
    "target_path": "rules/mamotama.conf",
    "rule_line": "SecRule REQUEST_URI \"@beginsWith /search\" \"id:190123,phase:1,pass,nolog,ctl:ruleRemoveTargetById=100004;ARGS:q,msg:'mamotama fp_tuner scoped exclusion'\""
  }
}
```

## 2) Apply

### Request

```json
{
  "proposal": {
    "id": "fp-mock-001",
    "target_path": "rules/mamotama.conf",
    "rule_line": "SecRule REQUEST_URI \"@beginsWith /search\" \"id:190123,phase:1,pass,nolog,ctl:ruleRemoveTargetById=100004;ARGS:q,msg:'mamotama fp_tuner scoped exclusion'\""
  },
  "simulate": true
}
```

Notes:
- `simulate` defaults to `true`.
- `rule_line` is validated against a strict allow-list pattern.

### Response (simulate)

```json
{
  "ok": true,
  "contract_version": "fp_tuner.v1",
  "simulated": true,
  "hot_reloaded": false,
  "reloaded_file": "rules/mamotama.conf",
  "preview_etag": "W/\"sha256:...\""
}
```

### Response (real apply)

```json
{
  "ok": true,
  "contract_version": "fp_tuner.v1",
  "etag": "W/\"sha256:...\"",
  "hot_reloaded": true,
  "reloaded_file": "rules/mamotama.conf"
}
```

## Security Behavior

- Provider request payload is sanitized before external send.
- Masked categories include bearer/jwt-like tokens, email, IPv4, and common secret query keys.
- Only scoped exclusion format is accepted for apply.
