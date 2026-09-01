# 13a — Development server preparation

**Date:** 2026-09-01 · Two parts: **A** is done once per server, **B** onboards
this project onto a server already prepared. Absolute paths throughout; editor
is `vi`.

`claudedev` has already had Part A done. Skip to Part B.

---

# Part A — the server, once

## A1. Base

```bash
sudo apt-get update && sudo apt-get -y upgrade
sudo apt-get -y install build-essential git curl ca-certificates ufw \
                        nginx postgresql postgresql-contrib python3
sudo timedatectl set-timezone Asia/Jakarta
```

## A2. Go and Node

```bash
sudo rm -rf /usr/local/go
curl -fsSL https://go.dev/dl/go1.26.5.linux-amd64.tar.gz | sudo tar -C /usr/local -xz
echo 'export PATH=$PATH:/usr/local/go/bin' | sudo tee /etc/profile.d/go.sh
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt-get install -y nodejs
```

## A3. PostgreSQL

Native, shared across projects — **one database per project** plus a `_test`
database. Do not run a second PostgreSQL in Docker.

```bash
sudo -u postgres psql -c 'SELECT version();'
sudo vi /etc/postgresql/*/main/postgresql.conf   # listen_addresses = 'localhost'
sudo systemctl restart postgresql
```

## A4. Satellites, in Docker

MinIO, mailpit and WAHA are shared containers, bound to loopback:

```bash
docker ps --format 'table {{.Names}}\t{{.Ports}}'
```

## A5. Firewall

```bash
sudo ufw default deny incoming
sudo ufw allow 22/tcp
sudo ufw enable
```

Per-project ports are opened in Part B — and **to every network the users
actually arrive from**, which is rarely just one.

---

# Part B — onboard marketing_calendar

## B1. Code

```bash
sudo mkdir -p /home/dev/projects
cd /home/dev/projects
git clone git@github.com:stevenwilliam/marketing_calendar.git
cd /home/dev/projects/marketing_calendar
```

## B2. Databases

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
sudo -u postgres psql -d marketing_calendar -c '\dx'
```

Extensions need superuser; the application role cannot create them.

## B3. Configuration

```bash
sudo mkdir -p /etc/marketing_calendar
sudo vi /etc/marketing_calendar/marketing_calendar.env
sudo chown root:dev /etc/marketing_calendar/marketing_calendar.env
sudo chmod 0640     /etc/marketing_calendar/marketing_calendar.env
```

Generate this project's own secrets — never copy another project's signing key:

```bash
openssl rand -base64 48   # JWT_SIGNING_KEY
openssl rand -base64 32   # TOTP_ENCRYPTION_KEY
```

## B4. Pick a port, having checked

```bash
ss -tln | grep -E ':(80|443|80[0-9][0-9])\b'
```

Other projects already hold ports on this server. Record the chosen port in
`11-local-dev-setup.md` §5 on the day it is chosen, and do not assume one is
free because it was last month.

## B5. Build, migrate, seed

```bash
cd /home/dev/projects/marketing_calendar
go build -o /home/dev/projects/marketing_calendar/bin/mc ./cmd/api
/home/dev/projects/marketing_calendar/bin/mc migrate
/home/dev/projects/marketing_calendar/bin/mc migrate:status
/home/dev/projects/marketing_calendar/bin/mc seed
```

## B6. systemd and nginx

```bash
sudo cp /home/dev/projects/marketing_calendar/deploy/marketing-calendar.service \
        /etc/systemd/system/
sudo systemctl daemon-reload && sudo systemctl enable --now marketing-calendar
systemctl is-active marketing-calendar

sudo cp /home/dev/projects/marketing_calendar/deploy/nginx-marketing-calendar.conf \
        /etc/nginx/sites-available/marketing-calendar
sudo ln -sf /etc/nginx/sites-available/marketing-calendar /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
```

## B7. Open the firewall — to every network users arrive from

```bash
sudo ufw allow from 192.168.88.0/24 to any port <PORT> proto tcp \
     comment 'marketing_calendar (LAN)'
sudo ufw allow from 172.16.0.0/24   to any port <PORT> proto tcp \
     comment 'marketing_calendar (VMware host net)'
sudo ufw status | grep <PORT>
```

> **Both rules.** Steven's machine does not arrive from the physical LAN — it
> comes through the VMware host adapter as `172.16.0.1`. A rule scoped only to
> the LAN looks correct and drops every packet. This has now cost time on two
> projects on this server; if it happens again, check `/var/log/ufw.log` for
> `DPT=<PORT>` before touching anything else.

## B8. Verify from another machine

```bash
curl -sS -o /dev/null -w '%{http_code}\n' http://<server>:<PORT>/healthz
curl -sS http://<server>:<PORT>/readyz
```

**Not from the server.** `curl` on the box bypasses the firewall entirely and
will report a healthy service while every user is blocked.
