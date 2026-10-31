#!/usr/bin/env python3
"""Install infrastructure guard after isolated verification; rollback on failure."""
import datetime
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import urllib.error
import urllib.request


def run(*args):
    result = subprocess.run(args, text=True, capture_output=True)
    if result.returncode:
        raise RuntimeError("command failed: " + " ".join(args) + "\n" + result.stderr[-1500:])
    return result.stdout


def get(path, port=443):
    url = ("https://aitokey.top" if port == 443 else "http://127.0.0.1:" + str(port)) + path
    request = urllib.request.Request(url, headers={"Host": "aitokey.top", "User-Agent": "Sub2API-guard-verification"})
    try:
        with urllib.request.urlopen(request, timeout=15) as response:
            return response.status
    except urllib.error.HTTPError as error:
        return error.code


def main():
    source = Path(__file__).resolve().parent
    run("python3", str(source / "test_access_alert.py"))
    run("python3", str(source / "verify_guard.py"))
    config = Path("/etc/caddy/Caddyfile")
    original = config.read_bytes()
    snippet = Path("/etc/caddy/sub2api-access-guard.caddy")
    if "sub2api-access-guard.caddy" in original.decode():
        raise RuntimeError("guard already installed; review rather than overwrite")
    stamp = datetime.datetime.now(datetime.timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    backup = Path("/opt/sub2api/deploy/.security-backups") / stamp
    backup.mkdir(parents=True, mode=0o700)
    shutil.copy2(config, backup / "Caddyfile")
    existing_files = [snippet, Path("/usr/local/lib/sub2api-security/access_alert.py"),
                      Path("/etc/systemd/system/sub2api-access-alert.service"),
                      Path("/etc/systemd/system/sub2api-access-alert.timer")]
    for file in existing_files:
        if file.exists():
            raise RuntimeError("existing infrastructure file needs review: " + str(file))
    baseline_paths = ["/", "/health", "/login", "/admin", "/v1/models", "/v1/usage",
                      "/api/v1/auth/me", "/api/v1/payment/webhook/easypay",
                      "/api/v1/auth/oauth/github/callback", "/.well-known/acme-challenge/guard-probe"]
    baseline = {path: get(path) for path in baseline_paths}
    if baseline["/health"] != 200:
        raise RuntimeError("public health is not healthy before deployment")
    active = dict(line.split("=", 1) for line in Path("/opt/sub2api/deploy/.blue-green-active").read_text().splitlines() if "=" in line)
    container = active["container"]
    app_before = run("docker", "inspect", "-f", "{{.State.StartedAt}}|{{.RestartCount}}|{{.State.Health.Status}}", container).strip()
    run("install", "-d", "-o", "caddy", "-g", "caddy", "-m", "0750", "/var/log/caddy")
    log_path = Path("/var/log/caddy/sub2api-access.jsonl")
    run("install", "-o", "caddy", "-g", "caddy", "-m", "0640", "/dev/null", str(log_path))
    run("install", "-m", "0644", str(source / "access-guard.caddy"), str(snippet))
    run("install", "-d", "-m", "0755", "/usr/local/lib/sub2api-security")
    run("install", "-m", "0644", str(source / "access_alert.py"), "/usr/local/lib/sub2api-security/access_alert.py")
    for unit in ("sub2api-access-alert.service", "sub2api-access-alert.timer"):
        run("install", "-m", "0644", str(source / unit), "/etc/systemd/system/" + unit)
    candidate = config.with_name("Caddyfile.security-candidate")
    updated, count = re.subn(r"(?m)^aitokey[.]top\s*\{", "aitokey.top {\n\timport /etc/caddy/sub2api-access-guard.caddy", original.decode(), count=1)
    if count != 1:
        raise RuntimeError("production site block not found")
    candidate.write_text(updated)
    os.chmod(candidate, 0o644)
    run("runuser", "-u", "caddy", "--", "caddy", "validate", "--config", str(candidate), "--adapter", "caddyfile")
    run("systemd-analyze", "verify", "/etc/systemd/system/sub2api-access-alert.service", "/etc/systemd/system/sub2api-access-alert.timer")
    try:
        candidate.replace(config)
        run("systemctl", "reload", "caddy")
        after = {path: get(path) for path in baseline_paths}
        if baseline != after:
            raise RuntimeError("business route response status changed")
        blocked = {path: get(path) for path in ["/.env", "/.env.production", "/.git/config", "/.aws/credentials", "/wp-admin/install.php", "/chosen.php"]}
        if any(code != 404 for code in blocked.values()):
            raise RuntimeError("scan rejection failed")
        # Synthetic canaries verify redaction on the live log without real credentials.
        req = urllib.request.Request("https://aitokey.top/?token=guard-live-secret-query", headers={
            "Authorization": "Bearer guard-live-secret-auth", "X-Api-Key": "guard-live-secret-key",
            "Cookie": "session=guard-live-secret-cookie", "Referer": "https://example.invalid/?guard-live-secret-referrer"})
        with urllib.request.urlopen(req, timeout=15) as response:
            response.read()
        with log_path.open() as stream:
            rows = [json.loads(line) for line in stream]
        if "guard-live-secret" in json.dumps(rows):
            raise RuntimeError("live log credential redaction failed")
        if not any(row.get("security_rule") == "sensitive_path" for row in rows):
            raise RuntimeError("blocked paths are not logged")
        run("systemctl", "daemon-reload")
        run("systemctl", "enable", "--now", "sub2api-access-alert.timer")
        run("systemctl", "start", "sub2api-access-alert.service")
        app_after = run("docker", "inspect", "-f", "{{.State.StartedAt}}|{{.RestartCount}}|{{.State.Health.Status}}", container).strip()
        if app_after != app_before:
            raise RuntimeError("application health/start state changed")
        report = {"backup": str(backup), "business_status_before_after": after,
                  "blocked_status": blocked, "live_log_redaction": "passed",
                  "application_unchanged": app_after, "timer": run("systemctl", "is-active", "sub2api-access-alert.timer").strip(),
                  "alert_mode": "alert_only"}
        (backup / "verification.json").write_text(json.dumps(report, indent=2))
        print(json.dumps(report))
    except Exception:
        run("systemctl", "disable", "--now", "sub2api-access-alert.timer")
        shutil.copy2(backup / "Caddyfile", config)
        run("systemctl", "reload", "caddy")
        raise


if __name__ == "__main__":
    main()
