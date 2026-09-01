# Run when you're back — interactive steps

Steps needing an interactive terminal, a browser, or credentials that do not
exist yet. Editor is `vi`; every path is absolute.

_Updated: 2026-09-01 — created with the document set. **No code exists yet**,
so most of this is preparation rather than repair._

---

## A. Nothing is blocking today

There is no running service to fix. This file exists so the steps are written
down before they are needed, rather than guessed at later.

## B. Before M1 — create the databases

Needs `sudo` and the postgres superuser, so it cannot be scripted from a
non-interactive session.

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

Extensions need superuser; migration `0001` fails without them.

## C. Before M1 — the config file and its secrets

```bash
sudo mkdir -p /etc/marketing_calendar
sudo vi /etc/marketing_calendar/marketing_calendar.env
sudo chown root:dev /etc/marketing_calendar/marketing_calendar.env
sudo chmod 0640     /etc/marketing_calendar/marketing_calendar.env
```

Generate this project's own keys. **Never copy another project's signing key** —
a shared key makes a token minted for one system valid in the other:

```bash
openssl rand -base64 48   # JWT_SIGNING_KEY
openssl rand -base64 32   # TOTP_ENCRYPTION_KEY
```

## D. Before M1 — choose a port

```bash
ss -tln | grep -E ':(80|443|80[0-9][0-9])\b'
```

`:8090` and `:8091` are taken on `claudedev` by other projects. Record the
choice in `11-local-dev-setup.md` §5.

## E. Before M15 — the firewall, to BOTH networks

```bash
sudo ufw allow from 192.168.88.0/24 to any port <PORT> proto tcp \
     comment 'marketing_calendar (LAN)'
sudo ufw allow from 172.16.0.0/24   to any port <PORT> proto tcp \
     comment 'marketing_calendar (VMware host net)'
sudo ufw status | grep <PORT>
```

Steven's machine arrives as `172.16.0.1` through the VMware host adapter, not
from the physical LAN. A rule scoped to one subnet looks right and drops every
packet from the other. This has cost time on two projects on this server.

Then **verify from another machine**, never with `curl` on the server — `curl`
on the box does not traverse the firewall.

If it still does not load:

```bash
sudo grep -a 'DPT=<PORT>' /var/log/ufw.log | tail -20
```

A line with `SRC=` naming the machine that cannot connect is the answer.

## F. Ongoing — the holiday calendar

Each December, load next year's Indonesian public holidays. The lead-time
calculation is wrong without them, and wrong invisibly — the number still
looks like seven (BR-3.3, D22).

Idul Fitri moves each year and carries a multi-day national holiday. It is the
one that matters most and the one most likely to be missed.

## G. Ongoing — check mailpit after touching notifications

http://127.0.0.1:8025 — every email in development lands there and none
leaves. An email that was "sent" but never arrived is the classic silent
failure, and the only way to know is to look.

## H. Before go-live

- [ ] Penetration test (`12` §10)
- [ ] Restore a backup into a scratch database and open a report from it — a
      successful `pg_dump` is not a proven backup (`09` §12)
- [ ] Confirm TLS terminates before HSTS is enabled
- [ ] First admin account created by the documented one-time command, not a
      default password
