import requests
import matplotlib.pyplot as plt
import seaborn as sns
import subprocess
import time
from typing import Dict, Any, List

# ==========================================
# 1. CONSTANTS & FSM PARAMETERS FROM THE PAPER
# ==========================================
# Anchor Gap (Delta H) thresholds to trigger FSM transitions
THRESHOLD_SUSPICIOUS = 100
THRESHOLD_SOVEREIGN = 500

PROMETHEUS_URL = "http://localhost:9090/api/v1/query_range"
COMPOSE_FILE = "compose.yml"


# ==========================================
# 2. STANDARD ACADEMIC PLOT CONFIGURATION (IEEE/ACM)
# ==========================================
def setup_academic_plot_style():
    """Set up matplotlib formatting for academic papers (Vector PDF)."""
    sns.set_theme(style="whitegrid")
    plt.rcParams.update(
        {
            "font.family": "serif",
            "font.size": 12,
            "axes.labelsize": 14,
            "axes.titlesize": 16,
            "legend.fontsize": 12,
            "xtick.labelsize": 12,
            "ytick.labelsize": 12,
            "pdf.fonttype": 42,
            "ps.fonttype": 42,
            "lines.linewidth": 2.5,
        }
    )


# ==========================================
# 3. PROMETHEUS DATA FETCHING
# ==========================================
def fetch_prometheus_metric(
    query: str, start_time: float, end_time: float, step: str = "1s"
) -> List[Dict]:
    """
    Query time-series data from Prometheus (e.g., T_DA or Throughput).
    Returns a list of timestamps and values to be used in a Pandas DataFrame.
    """
    params = {"query": query, "start": start_time, "end": end_time, "step": step}

    try:
        response = requests.get(PROMETHEUS_URL, params=params)
        response.raise_for_status()
        data = response.json()["data"]["result"]
        if not data:
            print(f"Warning: No data found for query '{query}'.")
            return []

        # Extract the first data stream
        values = data[0]["values"]
        return [{"timestamp": float(v[0]), "value": float(v[1])} for v in values]

    except Exception as e:
        print(f"Error querying Prometheus: {e}")
        return []


# ==========================================
# 4. CHAOS ENGINEERING AUTOMATION (VIA PUMBA PROFILES)
# ==========================================
def trigger_chaos_profile(profile_name: str):
    """
    Kích hoạt một kịch bản lỗi mạng thông qua Pumba profile.
    """
    print(
        f"[{time.strftime('%X')}] Kích hoạt kịch bản hỗn loạn (Chaos): {profile_name}..."
    )
    cmd = ["docker-compose", "-f", COMPOSE_FILE, "--profile", profile_name, "up", "-d"]
    subprocess.run(cmd, check=True)


def stop_chaos_profile(profile_name: str):
    """
    Dừng kịch bản lỗi mạng, khôi phục hệ thống về trạng thái hoàn hảo 0ms.
    """
    print(
        f"[{time.strftime('%X')}] Dừng kịch bản {profile_name}, khôi phục mạng lưới..."
    )
    cmd = ["docker-compose", "-f", COMPOSE_FILE, "--profile", profile_name, "stop"]
    subprocess.run(cmd, check=True)


# --- High-level semantic wrappers for specific test scripts ---


def simulate_eclipse_attack():
    """Cô lập hoàn toàn Node 01 khỏi mạng lưới."""
    trigger_chaos_profile("chaos-eclipse")


def recover_from_eclipse_attack():
    """Khôi phục Node 01 để đánh giá quá trình Re-anchoring."""
    stop_chaos_profile("chaos-eclipse")


def simulate_network_latency():
    """Tăng độ trễ mạng (100ms) để đánh giá độ trễ đồng thuận."""
    trigger_chaos_profile("chaos-delay")


def stop_network_latency():
    """Khôi phục độ trễ về 0ms."""
    stop_chaos_profile("chaos-delay")


def simulate_node_crash():
    """Sập nguồn đột ngột Node 04 (Crash Fault)."""
    trigger_chaos_profile("chaos-crash")
