# 11 — Local development

**Date:** 2026-09-01 · Development happens on the shared dev server
`claudedev`, not a laptop. Absolute paths throughout; editor is `vi`.

---

## 1. Prerequisites

Already present on `claudedev`: Go (latest), Node 20, PostgreSQL, nginx, and
the shared Docker satellites (MinIO, mailpit, WAHA). Do **not** stand up a
second PostgreSQL in Docker.

```bash
go version && node -v && psql --version && docker ps --format '{{.Names}}'
```

## 2. Databases

```bash
sudo -u postgres createuser --pwprompt marketing_calendar
sudo -u postgres createdb -O marketing_calendar marketing_calendar
sudo -u postgres createdb -O marketing_calendar marketing_calendar_test
for db in marketing_calendar marketing_calendar_test; do
  sudo -u postgres psql -d "$db" \
    -c 'CREATE EXTENSION IF NOT EXISTS btree_gist;' \
    -c 'CREATE EXTENSION IF NOT EXISTS citext;' \
    -c 'CREATE EXTENSION IF NOT EXISTS pgcrypto;'
done
```

Extensions need superuser. Do this before the first `migrate`.

## 3. Configuration

```bash
cd /home/dev/projects/marketing_calendar
cp /home/dev/projects/marketing_calendar/.env.example \
   /home/dev/projects/marketing_calendar/.env
chmod 0600 /home/dev/projects/marketing_calendar/.env
vi /home/dev/projects/marketing_calendar/.env
```

Generate secrets locally; never reuse another project's signing key:

```bash
openssl rand -base64 48   # JWT_SIGNING_KEY
openssl rand -base64 32   # TOTP_ENCRYPTION_KEY
```

`.env` is git-ignored. `/etc/marketing_calendar/marketing_calendar.env` is the
deployed copy and wins over `.env` when both are present.

## 4. Everyday commands

```bash
cd /home/dev/projects/marketing_calendar

go build -o /home/dev/projects/marketing_calendar/bin/mc ./cmd/api

/home/dev/projects/marketing_calendar/bin/mc migrate
/home/dev/projects/marketing_calendar/bin/mc migrate:status
/home/dev/projects/marketing_calendar/bin/mc seed
/home/dev/projects/marketing_calendar/bin/mc serve
/home/dev/projects/marketing_calendar/bin/mc job auto-cancel-promos

go vet ./...
go test ./...
export $(grep -E '^TEST_DATABASE_URL=' .env) && go test ./test/... -shuffle=on

python3 /home/dev/projects/marketing_calendar/scripts/contrast.py
```

The frontend:

```bash
cd /home/dev/projects/marketing_calendar/web
npm install
npm run dev
npm run build
```

## 5. Ports

**Chosen 2026-09-02: the service binds `127.0.0.1:8093`, and nginx exposes it
on `:8094` for the LAN.** Browse to **`http://192.168.88.101:8094/`**.

The two ports are not a mistake. 8093 is the Go service and is loopback-only,
so nothing reaches it without nginx. 8094 is nginx's door: `ruuma` owns
`listen 80 default_server` on this box, so a request to the bare IP on port 80
lands on Ruuma Eatery, and `marketing-calendar.sfg.local` is not in DNS — which
left the application running and unreachable from a browser. A dedicated port
is the same shape evermore uses on 8090/8091, and it needs no DNS at all.

`:8094` is open in ufw **to `192.168.88.0/24` only**, and the nginx allowlist
applies on top. In production both go away: the hostname resolves, TLS is on
443, and a second unencrypted door is not wanted.

Pick a free port and record it here on the day it is chosen. `:8090` and
`:8091` are taken by other projects on this server, so **check before
assuming**:

```bash
ss -tln | grep -E ':(80|443|809[0-9])\b'
```

The Go service binds `127.0.0.1`; nginx is the only way in.

## 6. Satellites

| Service | Where | Use |
|---|---|---|
| mailpit | `127.0.0.1:1025` SMTP, `:8025` UI | every email in dev lands here, none leaves |
| MinIO | `127.0.0.1:9002` | object storage |
| WAHA | `127.0.0.1:3000` | WhatsApp, phase 2 |

Check the mailpit UI after any change to notifications. An email that was
"sent" but never arrived is the classic silent failure.

## 7. Test database discipline

The integration suite truncates `marketing_calendar_test` to a known state in
`TestMain`, driven off the catalogue so a table added later is cleaned without
anyone remembering.

**The suite must pass under `-shuffle=on`, repeatedly.** Order dependence is a
defect in the suite. Run it several times before believing a green.

## 8. Before you commit

1. `go vet ./...`
2. `go test ./...` — read the output
3. `go test ./test/... -shuffle=on` a few times
4. `python3 scripts/contrast.py` if any colour changed
5. `tools/shot` against the running service if any screen changed
6. Docs updated **in the same commit**
