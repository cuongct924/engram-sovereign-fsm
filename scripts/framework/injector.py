"""Real Pumba chaos-engineering injection against the live docker testnet --
wraps the profiles scaffolded in root compose.yml (chaos-delay/loss/crash/
eclipse/btc-delay + WAN profiles) and runs them for real against the
engram-nodeNN containers.

Each pumba container's `command:` bakes in a --duration, so once started it
exits on its own -- no separate timer needed here, just polling for stop.
"""

import subprocess
import time
from dataclasses import dataclass

# Compose SERVICE names (what `docker compose --profile X up -d <service>`
# and `docker compose ... rm <service>` take).
PROFILE_TO_SERVICE = {
    "chaos-delay": "pumba-latency",
    "chaos-loss": "pumba-loss",
    "chaos-crash": "pumba-kill",
    "chaos-eclipse": "pumba-eclipse",
    "chaos-btc-delay": "pumba-btc-delay",
    "chaos-p2p-churn": "pumba-p2p-churn",
    "chaos-relay-latency": "pumba-relay-latency",
}

# Real CONTAINER names (compose.yml's `container_name:` override on each
# pumba service) -- what docker inspect/logs/wait take. These differ from
# the service names above (service "pumba-latency" is container
# "pumba-latency-injector"); using a service name for `docker inspect`
# returns "missing", reading as "already finished" before it started and
# letting a second netem profile start while the first's `tc qdisc` rule is
# still active (root qdiscs can't coexist -- "qdisc add ... exit code 2").
PROFILE_TO_CONTAINER = {
    "chaos-delay": "pumba-latency-injector",
    "chaos-loss": "pumba-loss-injector",
    "chaos-crash": "pumba-crash-injector",
    "chaos-eclipse": "pumba-eclipse-injector",
    "chaos-btc-delay": "pumba-btc-delay-injector",
    "chaos-p2p-churn": "pumba-p2p-churn-injector",
    "chaos-relay-latency": "pumba-relay-latency-injector",
}

# Real, human-readable description of each profile's actual command (see
# root compose.yml) -- kept here so callers/results can label what really
# ran without re-parsing YAML.
PROFILE_DESCRIPTIONS = {
    "chaos-delay": "netem delay 100ms +-20ms jitter on all engram-node* for 5m",
    "chaos-loss": "netem 5% packet loss on engram-node01,engram-node02 for 2m",
    "chaos-crash": "SIGKILL engram-node04 (immediate, one-shot)",
    "chaos-eclipse": "netem 100% packet loss on engram-node01 for 3m",
    "chaos-btc-delay": "netem delay 500ms +-100ms jitter on bitcoin-node01 for 2m",
    "chaos-p2p-churn": "netem 100% packet loss on attacker-a3-01 for 3m (toggled repeatedly)",
    "chaos-relay-latency": "netem delay 350ms +-50ms jitter on engram-node04 for 10m",
}


# WAN-realism profiles (compose.yml's "WAN realism profiles" section): one
# Pumba container per validator, each with a different delay/jitter or
# loss%; multi-service per profile, so service name == container_name (one
# list covers start and stop/rm).
#
# chaos-wan-latency and chaos-wan-loss must never run concurrently: both add
# a root netem qdisc on the same eth0, and a second root qdisc add fails
# outright (same conflict class as cleanup_profile's doc) -- mutually
# exclusive, not stackable.
WAN_PROFILE_TO_SERVICES = {
    "chaos-wan-latency": [
        "pumba-wan-latency-01",
        "pumba-wan-latency-02",
        "pumba-wan-latency-03",
        "pumba-wan-latency-04",
    ],
    "chaos-wan-loss": [
        "pumba-wan-loss-01",
        "pumba-wan-loss-02",
        "pumba-wan-loss-03",
        "pumba-wan-loss-04",
    ],
}


def start_pumba_wan_profile(profile: str, wait_running_timeout_s: float = 15.0) -> list:
    """Multi-service counterpart to start_pumba_profile -- starts all of
    profile's per-validator containers together, always with explicit service
    names (see cleanup_wan_profile's doc for why a bare --profile is
    dangerous)."""
    services = WAN_PROFILE_TO_SERVICES[profile]
    subprocess.run(
        ["docker", "compose", "--profile", profile, "up", "-d", *services],
        capture_output=True,
        text=True,
        timeout=30,
        check=True,
    )
    deadline = time.time() + wait_running_timeout_s
    while time.time() < deadline:
        if all(container_status(c) in ("running", "exited") for c in services):
            return services
        time.sleep(0.3)
    raise RuntimeError(
        f"{services} did not all reach running/exited state within {wait_running_timeout_s}s"
    )


def cleanup_wan_profile(profile: str) -> None:
    """Multi-service counterpart to cleanup_profile. ALWAYS passes explicit
    service names to stop/rm -- a bare `docker compose --profile X stop`
    stops the ENTIRE cluster, not just profile X's containers (--profile only
    widens up's defaults, it does not narrow stop/rm's scope).
    """
    services = WAN_PROFILE_TO_SERVICES[profile]
    subprocess.run(
        ["docker", "compose", "--profile", profile, "stop", *services],
        capture_output=True,
        text=True,
    )
    subprocess.run(
        ["docker", "compose", "--profile", profile, "rm", "-f", *services],
        capture_output=True,
        text=True,
    )
    for target in WAN_PROFILE_TO_NETEM_TARGETS.get(profile, []):
        _clear_netem_qdisc(target)


# Image used by _clear_netem_qdisc -- ships tc/iproute2 out of the box, so
# no extra install step is needed inside the throwaway debug container.
_NETEM_CLEANUP_IMAGE = "nicolaka/netshoot"

# Profile -> real container name(s) that get a `tc qdisc` applied directly on
# their own network namespace (not the pumba injector container). Needed
# because Pumba does not catch SIGTERM to remove its own netem rule --
# confirmed live: `docker stop` (graceful, well within the default grace
# period) left the qdisc on the target in place. A profile stopped before its
# own --duration elapses (every early-terminated live_*.py run, and every
# toggle_profile_bursts cycle) leaves the target permanently degraded until
# this runs explicitly. A leftover qdisc also makes the NEXT `tc qdisc add`
# on the same target fail outright ("exit code 2", RTNETLINK "File exists"),
# which Pumba treats as fatal and exits immediately without ever applying the
# new rule -- silently turning a whole attack phase into a no-op.
PROFILE_TO_NETEM_TARGETS = {
    "chaos-delay": ["engram-node01", "engram-node02", "engram-node03", "engram-node04"],
    "chaos-loss": ["engram-node01", "engram-node02"],
    "chaos-eclipse": ["engram-node01"],
    "chaos-btc-delay": ["bitcoin-node01"],
    "chaos-p2p-churn": ["attacker-a3-01"],
    "chaos-relay-latency": ["engram-node04"],
}

WAN_PROFILE_TO_NETEM_TARGETS = {
    "chaos-wan-latency": ["engram-node01", "engram-node02", "engram-node03", "engram-node04"],
    "chaos-wan-loss": ["engram-node01", "engram-node02", "engram-node03", "engram-node04"],
}


def _clear_netem_qdisc(container: str) -> None:
    """Best-effort `tc qdisc del dev eth0 root` on container's own network
    namespace, via a throwaway privileged debug container sharing its netns
    (container itself typically has no `tc` binary). Safe to call whether or
    not a qdisc is actually present -- `tc qdisc del` on a bare noqueue exits
    non-zero, which is ignored here since there's nothing to clean up."""
    subprocess.run(
        [
            "docker", "run", "--rm",
            "--net", f"container:{container}",
            "--cap-add", "NET_ADMIN",
            _NETEM_CLEANUP_IMAGE,
            "tc", "qdisc", "del", "dev", "eth0", "root",
        ],
        capture_output=True,
        text=True,
        timeout=30,
    )


@dataclass
class ChaosRunResult:
    profile: str
    service: str
    started_at: float
    ended_at: float
    returncode: int
    stdout: str
    stderr: str


def _up_detached(profile: str) -> str:
    """Starts ONLY the profile's own service (detached) and returns its real
    container name.

    `docker compose --profile X up` with no service name targets every
    service in the file -- services without a `profiles:` key are "always
    active", so the whole cluster would be (re)upped. Naming the service
    explicitly restricts the command to that one container.
    """
    if profile not in PROFILE_TO_SERVICE:
        raise ValueError(
            f"unknown profile {profile!r}, expected one of {list(PROFILE_TO_SERVICE)}"
        )
    service = PROFILE_TO_SERVICE[profile]
    subprocess.run(
        ["docker", "compose", "--profile", profile, "up", "-d", service],
        capture_output=True,
        text=True,
        timeout=30,
        check=True,
    )
    return PROFILE_TO_CONTAINER[profile]


def run_pumba_profile(profile: str, timeout_s: float = 600.0) -> ChaosRunResult:
    """Runs one Pumba profile to completion (blocking, via `docker wait` on
    the real container name) and removes the finished container afterward.
    """
    started = time.time()
    container = _up_detached(profile)
    wait_proc = subprocess.run(
        ["docker", "wait", container],
        capture_output=True,
        text=True,
        timeout=timeout_s,
    )
    logs_proc = subprocess.run(
        ["docker", "logs", container], capture_output=True, text=True
    )
    ended = time.time()

    exit_code = (
        int(wait_proc.stdout.strip()) if wait_proc.stdout.strip().isdigit() else -1
    )
    cleanup_profile(profile)

    return ChaosRunResult(
        profile=profile,
        service=PROFILE_TO_SERVICE[profile],
        started_at=started,
        ended_at=ended,
        returncode=exit_code,
        stdout=logs_proc.stdout,
        stderr=logs_proc.stderr,
    )


def start_pumba_profile(profile: str, wait_running_timeout_s: float = 15.0) -> str:
    """Non-blocking variant -- starts the profile's container and blocks only
    until it's observed running/exited, confirming it really registered
    before the caller polls is_running(). Returns the real container name."""
    container = _up_detached(profile)
    deadline = time.time() + wait_running_timeout_s
    while time.time() < deadline:
        status = container_status(container)
        if status in ("running", "exited"):
            return container
        time.sleep(0.3)
    raise RuntimeError(
        f"{container} did not reach running/exited state within {wait_running_timeout_s}s"
    )


def is_running(container: str) -> bool:
    return container_status(container) == "running"


def wait_for_no_active_netem(timeout_s: float = 20.0) -> None:
    """Confirms no other pumba netem container still holds a `tc qdisc` on
    any engram-node* target before starting a new netem profile (see
    PROFILE_TO_CONTAINER's doc for the failure this prevents). chaos-crash
    (SIGKILL, no qdisc) doesn't need it, but calling unconditionally is
    cheap and safe."""
    netem_containers = [
        "pumba-latency-injector",
        "pumba-loss-injector",
        "pumba-eclipse-injector",
        "pumba-btc-delay-injector",
        "pumba-p2p-churn-injector",
        "pumba-relay-latency-injector",
    ]
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        if all(container_status(c) in ("missing", "exited") for c in netem_containers):
            return
        time.sleep(0.5)
    still = [
        c for c in netem_containers if container_status(c) not in ("missing", "exited")
    ]
    raise RuntimeError(
        f"netem containers still active, refusing to start a new profile: {still}"
    )


def toggle_profile_bursts(profile: str, on_s: float, off_s: float, cycles: int) -> None:
    """Repeatedly starts and stops profile to simulate genuine peer
    connect/disconnect churn (docs/EXPERIMENT.md's E4 A3, E9's P2P churn-spike
    leg), built from the same start/cleanup primitives everything else uses.

    Each on-phase blocks until the container is confirmed running, sleeps
    on_s, then cleans up and sleeps off_s. Only meaningful for the netem
    profiles (chaos-delay/loss/eclipse/btc-delay), whose own --duration is
    long enough that natural expiry wouldn't produce repeated cycles within
    a reasonable window; chaos-crash (a one-shot SIGKILL) is not a valid input.
    """
    if profile == "chaos-crash":
        raise ValueError(
            "toggle_profile_bursts is for sustained netem profiles, not chaos-crash"
        )
    for i in range(cycles):
        print(
            f"toggle_profile_bursts({profile}): cycle {i + 1}/{cycles} -- ON for {on_s:.0f}s"
        )
        start_pumba_profile(profile)
        time.sleep(on_s)
        cleanup_profile(profile)
        print(
            f"toggle_profile_bursts({profile}): cycle {i + 1}/{cycles} -- OFF for {off_s:.0f}s"
        )
        time.sleep(off_s)
        wait_for_no_active_netem()


def cleanup_profile(profile: str) -> None:
    """Stops then removes profile's container. `docker compose rm -f` alone
    does NOT stop a still-running container first (unlike plain `docker rm
    -f`), so it silently no-ops while a profile's own --duration hasn't
    elapsed. This matters when interrupting a still-active profile early
    (e.g. toggle_profile_bursts holds chaos-loss for 20s of its 2m duration)
    -- without `stop` first the container stays "Up" and the next
    wait_for_no_active_netem() correctly refuses to start a new profile on
    top of it."""
    service = PROFILE_TO_SERVICE[profile]
    subprocess.run(
        ["docker", "compose", "--profile", profile, "stop", service],
        capture_output=True,
        text=True,
    )
    subprocess.run(
        ["docker", "compose", "--profile", profile, "rm", "-f", service],
        capture_output=True,
        text=True,
    )
    for target in PROFILE_TO_NETEM_TARGETS.get(profile, []):
        _clear_netem_qdisc(target)


def container_status(name: str) -> str:
    """Real docker inspect status ('running', 'exited', ...) for name, or
    'missing' if the container doesn't exist at all -- used to confirm a
    chaos-crash target actually died and/or came back via restart:
    unless-stopped, rather than assuming it from Pumba's own exit code.
    """
    proc = subprocess.run(
        ["docker", "inspect", "-f", "{{.State.Status}}", name],
        capture_output=True,
        text=True,
    )
    if proc.returncode != 0:
        return "missing"
    return proc.stdout.strip()


if __name__ == "__main__":
    # Manual CLI for `make chaos-<profile>`/`make chaos-stop` -- scripted
    # experiments (scripts/e*/live_*.py) call the functions above directly
    # and don't go through this.
    import sys

    if len(sys.argv) != 3 or sys.argv[1] not in ("start", "stop"):
        print(f"Usage: {sys.argv[0]} start|stop <profile>", file=sys.stderr)
        print(f"Profiles: {', '.join(PROFILE_TO_SERVICE)}", file=sys.stderr)
        sys.exit(1)
    action, profile = sys.argv[1], sys.argv[2]
    if profile not in PROFILE_TO_SERVICE:
        print(f"Unknown profile {profile!r}. Profiles: {', '.join(PROFILE_TO_SERVICE)}", file=sys.stderr)
        sys.exit(1)
    if action == "start":
        container = start_pumba_profile(profile)
        print(f"started {profile} ({container}) -- self-exits on its own --duration")
    else:
        cleanup_profile(profile)
        print(f"stopped {profile}")
