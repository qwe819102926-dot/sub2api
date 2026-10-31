#!/usr/bin/env python3
"""Read-only access-log detector. Never changes firewall or accounts."""
import argparse
import collections
import gzip
import ipaddress
import json
import logging
import logging.handlers
import os
from pathlib import Path
import time

TAIL_BYTES = 8 * 1024 * 1024


def read_recent(log_path, now):
    # Inspect the active file and archives just rotated in the detection window.
    # Tail limits keep repeated reads bounded; very large bursts may be truncated.
    candidates = [log_path]
    candidates += sorted(
        (p for p in log_path.parent.glob("sub2api-access-*.jsonl*")
         if p.stat().st_mtime >= now - 360),
        key=lambda p: p.stat().st_mtime, reverse=True,
    )[:2]
    for path in candidates:
        try:
            if path.suffix == ".gz":
                chunks = collections.deque(maxlen=TAIL_BYTES // 65536)
                with gzip.open(path, "rb") as source:
                    while chunk := source.read(65536):
                        chunks.append(chunk)
                data = b"".join(chunks)
            else:
                with path.open("rb") as source:
                    source.seek(0, 2)
                    size = source.tell()
                    source.seek(max(0, size - TAIL_BYTES))
                    if size > TAIL_BYTES:
                        source.readline()
                    data = source.read(TAIL_BYTES)
            for line in data.splitlines():
                try:
                    row = json.loads(line)
                    if now - 300 <= float(row.get("ts", 0)) <= now + 5:
                        yield row
                except (ValueError, TypeError):
                    continue
        except FileNotFoundError:
            continue  # A simultaneous log rotation is normal.


def detect(rows, now):
    scans = collections.defaultdict(set)
    failures = collections.Counter()
    for row in rows:
        request = row.get("request", {})
        # No forwarded-header trust: use the connected peer until proxies are
        # explicitly configured and verified. Never block an address here.
        address = request.get("client_ip") or request.get("remote_ip", "")
        try:
            if not ipaddress.ip_address(address).is_global:
                continue
        except ValueError:
            continue
        if row.get("security_rule") == "sensitive_path" and row["ts"] >= now - 60:
            # Only canonical categories go into alerts; paths may contain secrets.
            path = row.get("path", request.get("uri", "")).split("?", 1)[0]
            scans[address].add(path)
        if row.get("status") == 401:
            failures[address] += 1
    for address, paths in scans.items():
        if len(paths) >= 5:
            yield {"rule": "sensitive_path_scan", "ip": address,
                   "count": len(paths), "window_seconds": 60}
    for address, count in failures.items():
        if count >= 100:
            yield {"rule": "repeated_401", "ip": address,
                   "count": count, "window_seconds": 300}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--log", type=Path, default=Path("/var/log/caddy/sub2api-access.jsonl"))
    parser.add_argument("--state-dir", type=Path, default=Path("/var/lib/sub2api-access-alert"))
    args = parser.parse_args()
    os.umask(0o027)
    args.state_dir.mkdir(parents=True, exist_ok=True)
    state_path = args.state_dir / "state.json"
    try:
        state = json.loads(state_path.read_text())
    except FileNotFoundError:
        state = {}
    now = time.time()
    state = {key: stamp for key, stamp in state.items() if stamp > now - 86400}
    handler = logging.handlers.RotatingFileHandler(
        args.state_dir / "alerts.jsonl", maxBytes=2 * 1024 * 1024, backupCount=5,
        encoding="utf-8")
    logger = logging.getLogger("access-alert")
    logger.setLevel(logging.WARNING)
    logger.addHandler(handler)
    for event in detect(read_recent(args.log, now), now):
        key = event["rule"] + ":" + event["ip"]
        if now - state.get(key, 0) < 1800:
            continue
        event.update(ts=now, level="warning", action="alert_only")
        message = json.dumps(event, separators=(",", ":"))
        logger.warning(message)
        print(message, flush=True)  # systemd journal -> operational alert channel
        state[key] = now
    temporary = state_path.with_suffix(".tmp")
    temporary.write_text(json.dumps(state), encoding="utf-8")
    temporary.replace(state_path)
    handler.close()


if __name__ == "__main__":
    main()
