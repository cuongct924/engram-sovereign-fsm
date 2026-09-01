"""Scrapes E9's diagnostic-only `sensor_snapshot` lines from `docker logs` -- each validator's
own LOCAL sensor read (never committed state), so unlike every other E9 metric they can
legitimately differ between validators; callers must not average/reconcile them.

No timestamp to parse (height-keyed) -- a caller correlates height to wall-clock time via its
own RPC-sampled timeline instead.
"""

import re
import subprocess

SENSOR_SNAPSHOT_RE = re.compile(
    r"engramd: sensor_snapshot height=(\d+) btc_gap=(\d+) btc_spv_failed=(true|false) "
    r"da_gap=(\d+) da_failed=(true|false) attestation_failed=(true|false) "
    r"active_anchors=(\d+) clean_peers=(\d+) churn_rate=(\d+) avg_tenure=(\d+) peer_latency=(\d+)"
)


def scrape_sensor_snapshots(container: str) -> list:
    """Pulls and parses every sensor_snapshot line in `container`'s log history. Returns [] on
    any docker/log failure -- best-effort diagnostic data, never required.
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
    """Maps a scraped height to an approximate timestamp via the closest-height real RPC sample
    already collected for the same run. rpc_samples must be same-node with .height/.timestamp.
    """
    if not rpc_samples:
        return None
    closest = min(rpc_samples, key=lambda s: abs(s.height - height))
    return closest.timestamp
