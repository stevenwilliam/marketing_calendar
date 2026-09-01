# 09 — Production deployment

**Date:** 2026-09-01 · Copy-paste, empty machine, **absolute paths throughout**.
Editor is `vi`. Nothing here has been executed yet — this is the plan, and
`PROGRESS.md` will say when it has been run.

---

## 0. What this deploys

A single Go binary behind nginx, a native PostgreSQL, and Docker only for
satellites. **Not public facing:** nginx binds the internal network with an IP
allowlist and the Go service binds loopback.

## 1. The machine

Ubuntu LTS, 4 vCPU, 8 GB RAM, 100 GB SSD. One node — losing it loses the
service, and the protection is backups rather than redundancy (see `05` §6).

```bash
sudo apt-get update && sudo apt-get -y upgrade
sudo apt-get -y install nginx postgresql postgresql-contrib ufw \
                        ca-certificates curl git
sudo timedatectl set-timezone Asia/Jakarta
```

## 2. Firewall

```bash
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow 22/tcp
sudo ufw allow from 10.0.0.0/8    to any port 443 proto tcp comment 'mc (office LAN)'
sudo ufw allow from 172.16.0.0/24 to any port 443 proto tcp comment 'mc (VPN/host net)'
sudo ufw enable
sudo ufw status verbose
```

> **Both networks.** A rule scoped to one subnet looks correct and silently
> drops everyone arriving from the other. On the dev server this exact mistake
> cost an afternoon: nginx was listening, the app was healthy, and every packet
> from the VMware host adapter (`172.16.0.1`) was dropped before it arrived.
> **Verify from another machine — `curl` on the server never traverses the
> firewall.**

## 3. Database

```bash
sudo -u postgres createuser --pwprompt marketing_calendar
sudo -u postgres createdb -O marketing_calendar marketing_calendar
sudo -u postgres createdb -O marketing_calendar marketing_calendar_test
sudo -u postgres psql -d marketing_calendar \
     -c 'CREATE EXTENSION IF NOT EXISTS btree_gist;' \
     -c 'CREATE EXTENSION IF NOT EXISTS citext;' \
     -c 'CREATE EXTENSION IF NOT EXISTS pgcrypto;'
sudo -u postgres psql -d marketing_calendar -c '\dx'
```

Extensions need superuser; the application role cannot create them. Do this
before the first `migrate` or migration `0001` fails.

## 4. Application user and layout

```bash
sudo useradd --system --create-home --home-dir /srv/marketing_calendar \
             --shell /usr/sbin/nologin mcapp
sudo mkdir -p /srv/marketing_calendar/bin /srv/marketing_calendar/import
sudo mkdir -p /etc/marketing_calendar
sudo chown -R mcapp:mcapp /srv/marketing_calendar
```

## 5. Configuration

```bash
sudo vi /etc/marketing_calendar/marketing_calendar.env
sudo chown root:mcapp /etc/marketing_calendar/marketing_calendar.env
sudo chmod 0640      /etc/marketing_calendar/marketing_calendar.env
```

Keys are documented in `/srv/marketing_calendar/.env.example`. Generate the
secrets on the machine — never reuse another project's signing key:

```bash
openssl rand -base64 48   # JWT_SIGNING_KEY
openssl rand -base64 32   # TOTP_ENCRYPTION_KEY
```

## 6. Build and install

```bash
cd /srv/marketing_calendar
sudo -u mcapp git clone https://github.com/stevenwilliam/marketing_calendar.git .
sudo -u mcapp go build -trimpath \
     -ldflags "-s -w -X main.buildCommit=$(git rev-parse --short HEAD)" \
     -o /srv/marketing_calendar/bin/mc ./cmd/api
```

## 7. Migrate before serving

```bash
sudo -u mcapp env $(grep -v '^#' /etc/marketing_calendar/marketing_calendar.env | xargs) \
     /srv/marketing_calendar/bin/mc migrate
sudo -u mcapp env $(grep -v '^#' /etc/marketing_calendar/marketing_calendar.env | xargs) \
     /srv/marketing_calendar/bin/mc migrate:status
```

Migrations run **before** the new binary serves. `migrate:status` is the check
that the schema is where the code expects it.

## 8. systemd

```bash
sudo vi /etc/systemd/system/marketing-calendar.service
sudo systemctl daemon-reload
sudo systemctl enable --now marketing-calendar
systemctl status marketing-calendar --no-pager
```

The unit is in `deploy/marketing-calendar.service`: `Type=simple`, `User=mcapp`,
`EnvironmentFile=/etc/marketing_calendar/marketing_calendar.env`,
`Restart=on-failure`, and hardening — `NoNewPrivileges`, `ProtectSystem=strict`,
`ProtectHome=read-only`, an empty `CapabilityBoundingSet`,
`MemoryDenyWriteExecute`, `SystemCallFilter=@system-service`.

## 9. Scheduled jobs

```bash
sudo cp /srv/marketing_calendar/deploy/mc-jobs.{service,timer} /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now mc-jobs.timer
systemctl list-timers mc-jobs --no-pager
```

A systemd timer, not an in-process ticker: a restart cannot skip a day and two
instances cannot both run the auto-cancel.

## 10. nginx and TLS

```bash
sudo cp /srv/marketing_calendar/deploy/nginx-marketing-calendar.conf \
        /etc/nginx/sites-available/marketing-calendar
sudo ln -sf /etc/nginx/sites-available/marketing-calendar /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
```

Internal TLS: either the corporate CA, or certbot if the hostname resolves
publicly. HSTS is sent **only** once TLS actually terminates — sending it from a
plain-HTTP host pins browsers to a scheme the host does not serve.

The proxy sets `X-Forwarded-For`, and the app trusts it **only** from the
loopback proxy. Without that pinning, any client can forge the header and with
it the rate-limit key and the audit log's IP.

## 11. Verify — from another machine

```bash
curl -sS -o /dev/null -w '%{http_code}\n' https://mc.internal/healthz
curl -sS https://mc.internal/readyz
curl -sSI https://mc.internal/ | grep -iE 'strict-transport|content-security|x-frame'
```

Then log in, complete TOTP enrolment, and open the promotion report.

**Do not verify with `curl` on the server.** It bypasses the firewall entirely
and will report success while every real user is blocked.

## 12. Backups

```bash
sudo vi /usr/local/bin/mc-backup.sh
sudo chmod 0750 /usr/local/bin/mc-backup.sh
sudo vi /etc/systemd/system/mc-backup.{service,timer}
sudo systemctl enable --now mc-backup.timer
```

Nightly `pg_dump -Fc` to `/srv/backup/marketing_calendar/`, 30 daily plus 12
monthly, copied off-host.

**A backup is not proven by a successful dump.** Monthly: restore into a scratch
database, run `migrate:status`, and open a report for a month that had data.
An untested backup is a hope.

## 13. Rollback

```bash
sudo systemctl stop marketing-calendar
cd /srv/marketing_calendar && sudo -u mcapp git checkout <previous-tag>
sudo -u mcapp go build -trimpath -o /srv/marketing_calendar/bin/mc ./cmd/api
sudo systemctl start marketing-calendar
```

Migrations are **forward-only in production**. Rolling the binary back over a
newer schema works only if the migration was additive — which is why additive
migrations are the default and a destructive one is a deliberate, announced act.
