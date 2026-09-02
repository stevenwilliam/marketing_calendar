# 13 — Production deployment handbook

**Copy-paste, from an empty Ubuntu machine to a running service.**
Every path is absolute. The editor is `vi`. Every command is run as a user with
`sudo`; where a command must be run as another user, it says so.

**Date:** 2026-09-02 · Verified on the dev server `claudedev`, which is the
same shape as production. Steps marked **⚠ NOT YET RUN IN PRODUCTION** have
been executed on the dev server only — they are correct as written and have not
been executed against a production machine, because none exists yet.

---

## 0. What you are building

| Piece | Where |
|---|---|
| Go service `mc` | `/home/dev/projects/marketing_calendar/bin/mc`, binds **127.0.0.1 only** |
| Configuration | `/etc/marketing_calendar/marketing_calendar.env`, root-owned, mode 640 |
| PostgreSQL | native, database `marketing_calendar` (+ `marketing_calendar_test`) |
| nginx | reverse proxy, **IP allowlist**, TLS, the only way in |
| systemd | one service, two timers |
| Import drop | `/srv/marketing_calendar/import` |

The frontend is **embedded in the binary**. There is no separate web root and
no window in which the API is new and the assets are old.

---

## 1. The machine

```bash
sudo apt-get update
sudo apt-get install -y ca-certificates curl git ufw nginx postgresql \
    postgresql-contrib tzdata

# Business-day logic converts to Asia/Jakarta explicitly, but the journal is
# easier to read when the host agrees.
sudo timedatectl set-timezone Asia/Jakarta
timedatectl | head -3
```

Only 80 and 443 are open. The Go service is never exposed.

```bash
sudo ufw allow OpenSSH
sudo ufw allow 'Nginx Full'
sudo ufw --force enable
sudo ufw status verbose
```

---

## 2. PostgreSQL

```bash
sudo -u postgres psql -c "CREATE ROLE mc_app LOGIN PASSWORD 'CHANGE-ME-STRONG';"
sudo -u postgres psql -c "CREATE DATABASE marketing_calendar OWNER mc_app;"
sudo -u postgres psql -c "CREATE DATABASE marketing_calendar_test OWNER mc_app;"
```

Generate the password rather than inventing one:

```bash
python3 -c "import secrets,string; print(''.join(secrets.choice(string.ascii_letters+string.digits) for _ in range(32)))"
```

Confirm the role can connect **before** going further. A migration that fails
on authentication is a confusing way to learn this:

```bash
psql "postgres://mc_app:CHANGE-ME-STRONG@127.0.0.1:5432/marketing_calendar?sslmode=disable" -c "SELECT current_user, current_database();"
```

---

## 3. The code and the build

```bash
sudo mkdir -p /home/dev/projects
sudo chown dev:dev /home/dev/projects
cd /home/dev/projects
git clone git@github.com:stevenwilliam/marketing_calendar.git
cd /home/dev/projects/marketing_calendar
```

Go and Node, if the machine does not already have them:

```bash
# Go — check https://go.dev/dl for the current release before pinning.
curl -fsSLo /tmp/go.tgz https://go.dev/dl/go1.26.5.linux-amd64.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf /tmp/go.tgz
echo 'export PATH=$PATH:/usr/local/go/bin' | sudo tee /etc/profile.d/go.sh
export PATH=$PATH:/usr/local/go/bin
go version

curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt-get install -y nodejs
node --version
```

Build the frontend **first** — it is embedded into the binary, so building the
binary before the assets embeds a placeholder page:

```bash
cd /home/dev/projects/marketing_calendar/web
npm ci
npm run build
ls -la /home/dev/projects/marketing_calendar/web/dist/index.html

cd /home/dev/projects/marketing_calendar
go build -o /home/dev/projects/marketing_calendar/bin/mc ./cmd/api
/home/dev/projects/marketing_calendar/bin/mc
```

The last command prints the usage banner. If it prints a configuration error
instead, that is section 4 — do it and come back.

---

## 4. Configuration

Nothing secret is in git. The real file is root-owned and readable only by the
service user.

```bash
sudo mkdir -p /etc/marketing_calendar
sudo vi /etc/marketing_calendar/marketing_calendar.env
```

```ini
MC_ENV=production
MC_HTTP_ADDR=127.0.0.1:8093
MC_DATABASE_URL=postgres://mc_app:CHANGE-ME-STRONG@127.0.0.1:5432/marketing_calendar?sslmode=disable
MC_TEST_DATABASE_URL=postgres://mc_app:CHANGE-ME-STRONG@127.0.0.1:5432/marketing_calendar_test?sslmode=disable
MC_JWT_SECRET=PASTE-A-48-BYTE-SECRET-HERE
MC_TOTP_ISSUER="Marketing Calendar"
MC_SMTP_HOST=127.0.0.1
MC_SMTP_PORT=25
MC_SMTP_FROM=marketing-calendar@sfg.co.id
MC_NOTIFY_ENABLED=true
MC_IMPORT_DROP_PATH=/srv/marketing_calendar/import
MC_LOG_LEVEL=info
MC_METRICS_ENABLED=true
MC_TRUSTED_PROXIES=127.0.0.1
```

Generate the JWT secret; the service refuses to start below 32 characters:

```bash
python3 -c "import secrets; print(secrets.token_urlsafe(48))"
```

```bash
sudo chown root:dev /etc/marketing_calendar/marketing_calendar.env
sudo chmod 640 /etc/marketing_calendar/marketing_calendar.env
sudo ls -la /etc/marketing_calendar/marketing_calendar.env
```

The drop directory:

```bash
sudo mkdir -p /srv/marketing_calendar/import
sudo chown dev:dev /srv/marketing_calendar/import
```

---

## 5. Migrations and seed

```bash
cd /home/dev/projects/marketing_calendar
set -a && . /etc/marketing_calendar/marketing_calendar.env && set +a

/home/dev/projects/marketing_calendar/bin/mc migrate status
/home/dev/projects/marketing_calendar/bin/mc migrate up
/home/dev/projects/marketing_calendar/bin/mc migrate status
```

Migrations are **forward-only in production**. `migrate down` exists for
development. Each applied migration records the SHA-256 of the file that ran,
and the service refuses to start if a file changed after it was applied — that
means the schema in front of you is not the schema the file describes.

**Reference data** — roles, permissions, the approval chain, Indonesian public
holidays, and `sys_parameters`:

```bash
/home/dev/projects/marketing_calendar/bin/mc seed
```

> **The seed also creates nine demo accounts with a shared password.** On a
> production machine, delete them once real accounts exist, or do not run the
> seed at all and load reference data another way. The demo password is in
> `internal/app/seed.go` and is **not a secret**.

Create the first real account instead:

```bash
/home/dev/projects/marketing_calendar/bin/mc user create
```

It asks for surel, nama, kata sandi (minimum 12 characters, not echoed), a role
code, and **one or more company codes**. There is no "all companies" value:
adding a fourth brand later must grant nobody anything until somebody decides.

---

## 6. systemd

```bash
sudo cp /home/dev/projects/marketing_calendar/deploy/marketing-calendar.service /etc/systemd/system/
sudo cp /home/dev/projects/marketing_calendar/deploy/marketing-calendar-autocancel.service /etc/systemd/system/
sudo cp /home/dev/projects/marketing_calendar/deploy/marketing-calendar-autocancel.timer /etc/systemd/system/
sudo cp /home/dev/projects/marketing_calendar/deploy/marketing-calendar-import.service /etc/systemd/system/
sudo cp /home/dev/projects/marketing_calendar/deploy/marketing-calendar-import.timer /etc/systemd/system/

sudo systemctl daemon-reload
sudo systemctl enable --now marketing-calendar.service
sudo systemctl enable --now marketing-calendar-import.timer
sudo systemctl enable --now marketing-calendar-autocancel.timer

sudo systemctl status marketing-calendar --no-pager
systemctl list-timers 'marketing-calendar*' --no-pager
```

Confirm it is listening on loopback **and nowhere else**:

```bash
ss -ltnp | grep 8093
curl -s http://127.0.0.1:8093/healthz
curl -s http://127.0.0.1:8093/readyz
```

`ss` must show `127.0.0.1:8093`. If it shows `0.0.0.0:8093`, `MC_HTTP_ADDR` is
wrong and the service is reachable without nginx.

Check the startup line masks its secrets:

```bash
sudo journalctl -u marketing-calendar -n 20 --no-pager | grep starting
```

The database URL must render as `postgres://mc_app:••••••@…` and the JWT
secret as `••••••`. If either is legible, stop and fix it before continuing —
the journal keeps it.

---

## 7. nginx and TLS

```bash
sudo cp /home/dev/projects/marketing_calendar/deploy/nginx-marketing-calendar.conf \
        /etc/nginx/sites-available/marketing-calendar
sudo vi /etc/nginx/sites-available/marketing-calendar
```

Two things must be edited before this is correct:

1. **`server_name`** — the real internal hostname.
2. **The allowlist.** The `allow` lines are the whole of "not public facing".
   Put the office ranges and the VPN range in, and leave `deny all` last.

```bash
sudo ln -sf /etc/nginx/sites-available/marketing-calendar /etc/nginx/sites-enabled/marketing-calendar
sudo nginx -t
sudo systemctl reload nginx
```

**⚠ NOT YET RUN IN PRODUCTION** — TLS. On an internal hostname certbot's HTTP-01
challenge needs the name to resolve publicly; use DNS-01, or an internal CA:

```bash
sudo apt-get install -y certbot python3-certbot-nginx
sudo certbot --nginx -d marketing-calendar.sfg.co.id
sudo certbot renew --dry-run
```

Then confirm the allowlist genuinely refuses an outsider. **Do this from
another machine, not from the server** — `curl localhost` proves nothing about
who else can reach it:

```bash
# From an allowed machine:
curl -s -o /dev/null -w '%{http_code}\n' https://marketing-calendar.sfg.co.id/healthz   # expect 200
# From a machine outside the allowlist:
curl -s -o /dev/null -w '%{http_code}\n' https://marketing-calendar.sfg.co.id/healthz   # expect 403
```

---

## 8. First login

1. Open the site from an allowed machine.
2. Sign in with the account from section 5.
3. The first login shows a TOTP secret. Scan it, then enter the six digits.
   **TOTP is mandatory** — there is no way to reach the application without it.
4. Change `notify.release_recipients` in **Pengaturan** to the real list.
   It is the *only* recipient list for the release email; chain actors are
   not appended.
5. Load next year's Indonesian public holidays in **Master data → Hari libur**
   before December. Without them the lead time is wrong around Idul Fitri in a
   way that still looks like seven days.

---

## 9. Backups

```bash
sudo mkdir -p /var/backups/marketing_calendar
sudo vi /usr/local/bin/mc-backup.sh
```

```bash
#!/bin/bash
set -euo pipefail
set -a; . /etc/marketing_calendar/marketing_calendar.env; set +a
STAMP=$(date +%Y%m%d-%H%M%S)
OUT=/var/backups/marketing_calendar/marketing_calendar-$STAMP.dump
pg_dump "$MC_DATABASE_URL" --format=custom --file="$OUT"
find /var/backups/marketing_calendar -name '*.dump' -mtime +30 -delete
echo "wrote $OUT"
```

```bash
sudo chmod 750 /usr/local/bin/mc-backup.sh
sudo /usr/local/bin/mc-backup.sh
```

Schedule it:

```bash
sudo vi /etc/systemd/system/mc-backup.service
```

```ini
[Unit]
Description=marketing_calendar nightly database backup
[Service]
Type=oneshot
ExecStart=/usr/local/bin/mc-backup.sh
```

```bash
sudo vi /etc/systemd/system/mc-backup.timer
```

```ini
[Unit]
Description=Run the marketing_calendar backup nightly
[Timer]
OnCalendar=*-*-* 03:30:00 Asia/Jakarta
Persistent=true
[Install]
WantedBy=timers.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now mc-backup.timer
```

> **A restore is not proven by a successful `pg_restore`.** It is proven by
> running `mc migrate status` against the restored database and opening the
> promotion report for a month that had data. Do that at least once, on a
> spare database, before you need it:
>
> ```bash
> sudo -u postgres createdb mc_restore_test
> pg_restore -d "postgres://mc_app:PASS@127.0.0.1:5432/mc_restore_test" /var/backups/marketing_calendar/<file>.dump
> MC_DATABASE_URL="postgres://mc_app:PASS@127.0.0.1:5432/mc_restore_test" \
>   /home/dev/projects/marketing_calendar/bin/mc migrate status
> sudo -u postgres dropdb mc_restore_test
> ```

---

## 10. Deploying a new version

Migrations run **before** the new binary serves, so the schema is never behind
the code.

```bash
cd /home/dev/projects/marketing_calendar
git pull

cd /home/dev/projects/marketing_calendar/web && npm ci && npm run build
cd /home/dev/projects/marketing_calendar
go build -o /home/dev/projects/marketing_calendar/bin/mc.new ./cmd/api

set -a && . /etc/marketing_calendar/marketing_calendar.env && set +a
/home/dev/projects/marketing_calendar/bin/mc.new migrate up

sudo systemctl stop marketing-calendar
mv /home/dev/projects/marketing_calendar/bin/mc /home/dev/projects/marketing_calendar/bin/mc.prev
mv /home/dev/projects/marketing_calendar/bin/mc.new /home/dev/projects/marketing_calendar/bin/mc
sudo systemctl start marketing-calendar

sleep 2
curl -s http://127.0.0.1:8093/readyz
sudo systemctl status marketing-calendar --no-pager | head -5
```

### Rollback

The binary rolls back in seconds. **The schema does not**: migrations are
forward-only, so a rollback is only safe when the new version added no
migration, or added one the old binary tolerates.

```bash
sudo systemctl stop marketing-calendar
mv /home/dev/projects/marketing_calendar/bin/mc.prev /home/dev/projects/marketing_calendar/bin/mc
sudo systemctl start marketing-calendar
curl -s http://127.0.0.1:8093/readyz
```

If the new version *did* migrate, restore from the backup taken before the
deploy rather than running `migrate down` on production data.

---

## 11. When something is wrong

```bash
sudo systemctl status marketing-calendar --no-pager
sudo journalctl -u marketing-calendar -n 100 --no-pager
sudo journalctl -u marketing-calendar -f
sudo journalctl -u marketing-calendar-import.service -n 50 --no-pager
sudo journalctl -u marketing-calendar-autocancel.service -n 50 --no-pager
```

| Symptom | Almost always |
|---|---|
| Service will not start, "konfigurasi wajib tidak ada" | `MC_DATABASE_URL` or `MC_JWT_SECRET` missing from the env file |
| "migrasi tertunda — jalankan `mc migrate up`" | Deployed the binary without running migrations |
| "berkas migrasi diubah setelah dijalankan" | Someone edited an applied migration. Do not force it; write a new migration |
| nginx 502 | The service is down, or `MC_HTTP_ADDR` does not match the `upstream` |
| 403 from a staff machine | Its address is not in the nginx allowlist |
| Login works, every screen is empty | The account has no company assigned. Grant one in **Pengguna** |
| Plans auto-cancelling too often | Five approvals against a 7-day lead time is roughly one step per day. Retune `promo.lead_time_working_days` or `promo.auto_cancel_days_before` in **Pengaturan** — no deploy needed |
| Release email nobody receives | `notify.release_recipients` is a fixed list and goes stale silently. Read it aloud at month-end |

A user reporting an error can quote the `trace_id` from the message; it appears
in the log line for that request.
