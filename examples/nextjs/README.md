# mamotama example: Next.js

This example places mamotama in front of a minimal Next.js app.

## Start

```bash
cd examples/nextjs
./setup.sh
docker compose up -d --build
```

- App URL: `http://localhost:${OPENRESTY_PORT:-18081}`
- Coraza API: `http://localhost:${CORAZA_PORT:-19091}/mamotama-api/status`

## Smoke tests

```bash
curl -i "http://localhost:18081/"
curl -i "http://localhost:18081/?q=<script>alert(1)</script>"
```

The second request should be blocked by WAF (`403`).
