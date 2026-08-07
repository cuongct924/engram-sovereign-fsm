"""Real Pumba chaos-engineering injection against the live docker testnet --
wraps the 4 profiles already scaffolded in the root compose.yml
(pumba-latency/loss/kill/eclipse, profiles chaos-delay/chaos-loss/chaos-crash/
chaos-eclipse). Those profiles were never actually run before this session
(M7 was previously blocked -- no Docker daemon available, see CLAUDE.md);
this module runs them for real, against the real engram-nodeNN containers.

Each pumba container's own `command:` already bakes in a --duration (e.g.
"netem --duration=5m ..."), so once started it exits on its own -- no
separate timer needed here, just polling for it to stop.
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
}

# Real CONTAINER names (compose.yml's `container_name:` override on each
# pumba service) -- what `docker inspect`/`docker logs`/`docker wait` take.
# These do NOT match the service names above (e.g. service "pumba-latency"
# has container_name "pumba-latency-injector") -- an earlier version of this
# module used the service name for `docker inspect` calls too, which always
# returned "missing" (no container is ever literally named "pumba-latency"),
# making every run look like Pumba had already finished before it started.
# Confirmed for real in this session: two experiments in a row logged zero
# "during" samples and, worse, that false "already finished" reading let a
# SECOND netem profile start while the first one's `tc qdisc` rule was still
# actually active, which then failed for real ("qdisc add ... failed with
# exit code 2" -- a root qdisc can't coexist with another root qdisc on the
# same interface).
PROFILE_TO_CONTAINER = {
    "chaos-delay": "pumba-latency-injector",
    "chaos-loss": "pumba-loss-injector",
    "chaos-crash": "pumba-crash-injector",
    "chaos-eclipse": "pumba-eclipse-injector",
}

# Real, human-readable description of each profile's actual command (see
# root compose.yml) -- kept here so callers/results can label what really
# ran without re-parsing YAML.
PROFILE_DESCRIPTIONS = {
    "chaos-delay": "netem delay 100ms +-20ms jitter on all engram-node* for 5m",
    "chaos-loss": "netem 5% packet loss on engram-node01,engram-node02 for 2m",
    "chaos-crash": "SIGKILL engram-node04 (immediate, one-shot)",
    "chaos-eclipse": "netem 100% packet loss on engram-node01 for 3m",
}


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

    IMPORTANT: `docker compose --profile X up` with NO service name targets
    every service in the file, not just the profile-gated one -- every
    engram-nodeNN/bitcoin/celestia/monitoring service here has no `profiles:`
    key at all, so compose treats them as "always active" and would try to
    (re)up all of them too (confirmed via `docker compose --profile
    chaos-delay config --services`, which lists all 10 services, not just
    pumba-latency). Naming the service explicitly (`up <service>`) restricts
    the command to that one container.
    """
    if profile not in PROFILE_TO_SERVICE:
        raise ValueError(f"unknown profile {profile!r}, expected one of {list(PROFILE_TO_SERVICE)}")
    service = PROFILE_TO_SERVICE[profile]
    subprocess.run(
        ["docker", "compose", "--profile", profile, "up", "-d", service],
        capture_output=True, text=True, timeout=30, check=True,
    )
    return PROFILE_TO_CONTAINER[profile]


def run_pumba_profile(profile: str, timeout_s: float = 600.0) -> ChaosRunResult:
    """Runs one Pumba profile to completion (blocking, via `docker wait` on
    the real container name) and removes the finished container afterward.
    """
    started = time.time()
    container = _up_detached(profile)
    wait_proc = subprocess.run(
        ["docker", "wait", container], capture_output=True, text=True, timeout=timeout_s,
    )
    logs_proc = subprocess.run(["docker", "logs", container], capture_output=True, text=True)
    ended = time.time()

    exit_code = int(wait_proc.stdout.strip()) if wait_proc.stdout.strip().isdigit() else -1
    cleanup_profile(profile)

    return ChaosRunResult(
        profile=profile, service=PROFILE_TO_SERVICE[profile], started_at=started, ended_at=ended,
        returncode=exit_code, stdout=logs_proc.stdout, stderr=logs_proc.stderr,
    )


def start_pumba_profile(profile: str, wait_running_timeout_s: float = 15.0) -> str:
    """Non-blocking variant -- starts the profile's container and blocks
    only until it's actually observed running/exited (confirms the
    container really registered before the caller starts polling
    is_running() -- see PROFILE_TO_CONTAINER's doc for why using the wrong
    name here silently broke this before). Returns the real container name
    for the caller to poll.
    """
    container = _up_detached(profile)
    deadline = time.time() + wait_running_timeout_s
    while time.time() < deadline:
        status = container_status(container)
        if status in ("running", "exited"):
            return container
        time.sleep(0.3)
    raise RuntimeError(f"{container} did not reach running/exited state within {wait_running_timeout_s}s")


def is_running(container: str) -> bool:
    return container_status(container) == "running"


def wait_for_no_active_netem(timeout_s: float = 20.0) -> None:
    """Confirms no OTHER pumba netem container is still holding a `tc qdisc`
    on any engram-node* target before starting a new netem-based profile --
    see PROFILE_TO_CONTAINER's doc for the real failure this prevents.
    chaos-crash (SIGKILL, no qdisc) doesn't strictly need this, but calling
    it unconditionally is cheap and safe.
    """
    netem_containers = ["pumba-latency-injector", "pumba-loss-injector", "pumba-eclipse-injector"]
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        if all(container_status(c) in ("missing", "exited") for c in netem_containers):
            return
        time.sleep(0.5)
    still = [c for c in netem_containers if container_status(c) not in ("missing", "exited")]
    raise RuntimeError(f"netem containers still active, refusing to start a new profile: {still}")


def cleanup_profile(profile: str) -> None:
    service = PROFILE_TO_SERVICE[profile]
    subprocess.run(["docker", "compose", "--profile", profile, "rm", "-f", service],
                    capture_output=True, text=True)


def container_status(name: str) -> str:
    """Real docker inspect status ('running', 'exited', ...) for name, or
    'missing' if the container doesn't exist at all -- used to confirm a
    chaos-crash target actually died and/or came back via restart:
    unless-stopped, rather than assuming it from Pumba's own exit code.
    """
    proc = subprocess.run(
        ["docker", "inspect", "-f", "{{.State.Status}}", name],
        capture_output=True, text=True,
    )
    if proc.returncode != 0:
        return "missing"
    return proc.stdout.strip()
