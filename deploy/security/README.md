# Production access guard

This guard is infrastructure configuration; importing it does not replace the
application image or change billing, API quotas, database settings or users.
The application access-logger identity enhancement is a separate image release.

## Installed layout

- `/etc/caddy/Caddyfile`: import `sub2api-access-guard.caddy` inside `aitokey.top`.
- `/etc/caddy/sub2api-access-guard.caddy`: redacted access logging and narrow path rules.
- `/var/log/caddy/sub2api-access.jsonl`: all completed requests, including blocked paths.
- `/usr/local/lib/sub2api-security/access_alert.py`: read-only detector.
- `sub2api-access-alert.timer`: runs once a minute as the `caddy` user.
- `/var/lib/sub2api-access-alert/alerts.jsonl`: deduplicated warnings.
- `/var/lib/sub2api-access-alert/state.json`: per-IP/rule notification cooldown.

Access logs omit request/response headers, bodies and query strings entirely.
The separate `path` field also redacts the asset token in `/api/v1/plugin-ui/`.
The response request ID links proxy records with backend records. User/key IDs
are established by backend authentication, not proxy-supplied identity headers.

The path rules reject `.env*`, `.git`, `.aws`, WordPress admin/content/includes
and `.php` script paths, including nested paths and PHP path-info. They do not
inspect message content or query parameters. ACME challenge paths are excluded.
The previous October 24 IP restrictions and active proxy upstream are preserved.

Access-log rotation: 50 MiB per file, up to 10 archives, maximum archive age 14
days; count and age are upper limits, not a guarantee of 14 days of history.
Alert logs rotate at 2 MiB with five archives. Files have restricted permissions.

## Detection and notification

- Five distinct blocked paths from one public peer in 60 seconds: scan warning.
- 100 responses with HTTP 401 from one public peer in 300 seconds: auth warning.
- Same rule and peer: one warning every 30 minutes while the condition persists.
- No automatic firewall, IP, account or API-key blocking.

HTTP 401 may result from stale credentials or a shared egress. A warning is not
proof of an attack. The detector reads the last 8 MiB of the active file and up
to two archives modified in the last six minutes. Extreme bursts may exceed
this bounded read window. It never logs a request path in the warning itself.

Warnings go to the service journal and `alerts.jsonl`. The Codex chat heartbeat
checks new warnings every 15 minutes and only notifies on new actionable events
or monitoring failures. Notification requires that local automation can execute
and reach SSH; detection continues on the server independently. This is not a
separate email or messaging integration.

The detector uses `client_ip` when configured, otherwise the directly connected
peer address. If a CDN is added, verify proxy trust and actual client attribution
before changing thresholds; never trust arbitrary forwarded headers.

## Verify and install

Prepare the files in an infrastructure candidate directory, then run
`python3 test_access_alert.py` and `python3 verify_guard.py` on the target host.
The latter runs another Caddy process with an isolated, disabled admin API,
random loopback ports, a mock upstream and temporary logs. It exercises path
matching, credential/query forwarding, log redaction, SSE and WebSocket upgrade.

`install_guard.py` runs these checks again, records baseline route statuses,
backs up the current Caddyfile under `.security-backups/`, validates the candidate
as `caddy`, and reloads Caddy. A post-reload failure restores the previous
Caddyfile and disables the timer. Application containers are not restarted.

Caddy configuration reload can close existing WebSocket connections; clients
may need to reconnect. Verification of an isolated WebSocket upgrade does not
establish that existing production connections survived a reload.

Operational checks:

```sh
systemctl is-active sub2api-access-alert.timer
systemctl is-failed sub2api-access-alert.service
journalctl -u sub2api-access-alert.service --since '1 hour ago' --no-pager
curl -fsS https://aitokey.top/health
```

## Rollback

Disable the alert timer. Remove only the access-guard import from a candidate
copy of the current production Caddyfile, validate it, then atomically replace
the Caddyfile and reload Caddy. Preserve the current reverse-proxy port and all
other restrictions. Retain log files for evidence. Use the original full backup
only for immediate rollback when no subsequent deployment has changed the route.

Future blue-green releases replace only the upstream port in the main Caddyfile,
so the import and security snippet survive normal application deployments.

## Initial installation evidence

Installed on 2026-10-31 at 13:32 China time. Backup:
`/opt/sub2api/deploy/.security-backups/20261031T053230Z`.
The application remained `sub2api-green-20261028_121816`, healthy, restart count 0.
Public `/health` remained 200; business route statuses matched the baseline.
Ten blocked-path cases, eleven pass-through cases, log redaction, SSE and
WebSocket upgrade passed isolated verification. Production blocked paths returned
404; production synthetic secrets were absent from the access log.
The scan warning for `154.44.9.107` at 13:32 is the installer self-test.
