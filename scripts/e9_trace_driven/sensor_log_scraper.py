"""Scrapes E9's diagnostic-only `sensor_snapshot` lines from `docker logs` --
x/sovereignty/sensors_refresh.go's RefreshMetrics prints one per block, per
validator, right before the derived (not raw) PeripheralMetrics gets
committed. These are each validator's own LOCAL sensor reads: never
committed state (`preblock.go`'s NewPreBlocker), so unlike every other E9
live metric, they can legitimately differ between validators -- that's
expected, not a bug, and callers must not average/reconcile them.

Same `docker logs` + regex pattern as
scripts/e8_attack_resilience/live_double_signing_test.py's
scan_logs_for_evidence -- no timestamp to parse (the line is height-keyed,
matching this package's existing engramd: diagnostic convention), so a
caller correlates height -> wall-clock time via its own RPC-sampled timeline
instead.
"""

import re
import subprocess

SENSOR_SNAPSHOT_RE = re.compile(
    r"engramd: sensor_snapshot height=(\d+) btc_gap=(\d+) btc_spv_failed=(true|false) "
    r"da_gap=(\d+) da_failed=(true|false) attestation_failed=(true|false) "
    r"active_anchors=(\d+) clean_peers=(\d+) churn_rate=(\d+) avg_tenure=(\d+) peer_latency=(\d+)"
)


def scrape_sensor_snapshots(container: str) -> list:
    """Pulls and parses every sensor_snapshot line seen in `container`'s
    entire log history so far -- one dict per block this validator ran
    RefreshMetrics on. Returns [] on any docker/log failure (best-effort
    diagnostic data, never something a caller should treat as required).
    """
    try:
        proc = subprocess.run(
            ["docker", "logs", container], capture_output=True, text=True, timeout=30
        )
    except (subprocess.TimeoutExpired, OSError):
        return []
    text = proc.stdout + proc.stderr
    out = []
    for m in SENSOR_SNAPSHOT_RE.finditer(text):
        (
            height,
            btc_gap,
            btc_spv_failed,
            da_gap,
            da_failed,
            attestation_failed,
            active_anchors,
            clean_peers,
            churn_rate,
            avg_tenure,
            peer_latency,
        ) = m.groups()
        out.append(
            {
                "node": container,
                "height": int(height),
                "btc_gap": int(btc_gap),
                "btc_spv_failed": btc_spv_failed == "true",
                "da_gap": int(da_gap),
                "da_failed": da_failed == "true",
                "attestation_failed": attestation_failed == "true",
                "active_anchors": int(active_anchors),
                "clean_peers": int(clean_peers),
                "churn_rate": int(churn_rate),
                "avg_tenure": int(avg_tenure),
                "peer_latency": int(peer_latency),
            }
        )
    return out


def nearest_timestamp(height: int, rpc_samples: list) -> float:
    """Maps a scraped block height to an approximate wall-clock timestamp by
    finding the closest-height real RPC sample (scripts/framework/logger.py's
    NodeSample, which carries both height and timestamp) already collected
    for the same run -- avoids needing a timestamp in the diagnostic log line
    at all. rpc_samples must be same-node samples with .height/.timestamp.
    """
    if not rpc_samples:
        return None
    closest = min(rpc_samples, key=lambda s: abs(s.height - height))
    return closest.timestamp
