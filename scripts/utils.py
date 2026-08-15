import matplotlib.pyplot as plt
import seaborn as sns
import subprocess
import time

# 1. FSM PARAMETERS: Anchor Gap (Delta H) thresholds triggering FSM transitions
THRESHOLD_SUSPICIOUS = 100
THRESHOLD_SOVEREIGN = 500

COMPOSE_FILE = "compose.yml"


# ==========================================
# 2. STANDARD ACADEMIC PLOT CONFIGURATION (IEEE/ACM)
# ==========================================
# IEEE/ACM two-column US-Letter widths (inches): 3.5in single-column,
# 7.16in double-column (`\begin{figure*}`); ACM's sigconf (7.0-7.1in) is
# close enough that either fits without rescaling.
FIG_WIDTH_SINGLE_COL = 3.5
FIG_WIDTH_DOUBLE_COL = 7.16
GOLDEN_RATIO = 1.618  # width:height for a single balanced plot
PRINT_DPI = 300  # PNG fallback; the .pdf (vector, fonts via pdf.fonttype=42) is the primary format


def figsize_single(width=FIG_WIDTH_SINGLE_COL, ratio=GOLDEN_RATIO):
    """(width, height) in inches for a single-column, single-panel plot."""
    return (width, width / ratio)


def figsize_multi_panel(n_panels, width=FIG_WIDTH_DOUBLE_COL, panel_height=1.4):
    """(width, height) for a double-column figure with n_panels stacked
    vertically (multi-row timeline); 1.4in keeps rows legible on a page.
    """
    return (width, panel_height * n_panels)


def figsize_row(
    n_subplots,
    width=FIG_WIDTH_DOUBLE_COL,
    height=FIG_WIDTH_DOUBLE_COL / (GOLDEN_RATIO * 1.4),
):
    """(width, height) for n_subplots side-by-side in one row (summary bars)."""
    return (width, height)


def figsize_grid(n_rows, n_cols, width=FIG_WIDTH_DOUBLE_COL):
    """(width, height) for an n_rows x n_cols grid, each cell near the golden
    ratio -- a 2x2 grid stays roughly square-ish.
    """
    return (width, (width / n_cols) * n_rows / GOLDEN_RATIO * 1.15)


def savefig_academic(fig, out_dir, basename):
    """Saves fig as <basename>.pdf (vector, primary) and <basename>.png
    (PRINT_DPI raster, fallback) -- the single place both formats/DPI are
    decided, keeping figure scripts consistent.
    """
    import os

    os.makedirs(out_dir, exist_ok=True)
    fig.savefig(os.path.join(out_dir, f"{basename}.pdf"))
    fig.savefig(os.path.join(out_dir, f"{basename}.png"), dpi=PRINT_DPI)


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
# 3. CHAOS ENGINEERING AUTOMATION (VIA PUMBA PROFILES)
# ==========================================
def trigger_chaos_profile(profile_name: str):
    """Trigger a network fault scenario via a Pumba profile."""
    print(
        f"[{time.strftime('%X')}] Triggering chaos scenario: {profile_name}..."
    )
    cmd = ["docker-compose", "-f", COMPOSE_FILE, "--profile", profile_name, "up", "-d"]
    subprocess.run(cmd, check=True)


def stop_chaos_profile(profile_name: str):
    """Stop a network fault scenario, restoring the system to 0ms."""
    print(
        f"[{time.strftime('%X')}] Stopping {profile_name}, restoring network..."
    )
    cmd = ["docker-compose", "-f", COMPOSE_FILE, "--profile", profile_name, "stop"]
    subprocess.run(cmd, check=True)


# --- High-level semantic wrappers for specific test scripts ---


def simulate_eclipse_attack():
    """Fully isolate Node 01 from the network."""
    trigger_chaos_profile("chaos-eclipse")


def recover_from_eclipse_attack():
    """Restore Node 01 to evaluate the Re-anchoring process."""
    stop_chaos_profile("chaos-eclipse")


def simulate_network_latency():
    """Raise network latency (100ms) to evaluate consensus latency."""
    trigger_chaos_profile("chaos-delay")


def stop_network_latency():
    """Restore latency to 0ms."""
    stop_chaos_profile("chaos-delay")


def simulate_node_crash():
    """Suddenly power-fail Node 04 (Crash Fault)."""
    trigger_chaos_profile("chaos-crash")
