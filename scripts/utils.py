import matplotlib.pyplot as plt
import seaborn as sns
import subprocess
import time

# ==========================================
# 1. CONSTANTS & FSM PARAMETERS FROM THE PAPER
# ==========================================
# Anchor Gap (Delta H) thresholds to trigger FSM transitions
THRESHOLD_SUSPICIOUS = 100
THRESHOLD_SOVEREIGN = 500

COMPOSE_FILE = "compose.yml"


# ==========================================
# 2. STANDARD ACADEMIC PLOT CONFIGURATION (IEEE/ACM)
# ==========================================
# IEEE/ACM two-column US-Letter template widths (inches) -- a figure that
# fits in one column vs. one that spans the full text width (`\begin{figure*}`).
# 3.5in single-column and 7.16in double-column are the standard values for
# IEEE's own conference/journal templates (7.16in = 182mm text width with
# standard 0.75in margins on 8.5x11 US Letter); ACM's sigconf template is
# close enough (7.0-7.1in) that either figure still fits without rescaling
# in the final layout.
FIG_WIDTH_SINGLE_COL = 3.5
FIG_WIDTH_DOUBLE_COL = 7.16
GOLDEN_RATIO = 1.618  # width:height for a single balanced plot
PRINT_DPI = 300  # print-quality minimum for the PNG fallback; the .pdf
# (vector, fonts embedded via pdf.fonttype=42 below) is the primary format
# for LaTeX inclusion and is resolution-independent regardless of DPI.


def figsize_single(width=FIG_WIDTH_SINGLE_COL, ratio=GOLDEN_RATIO):
    """(width, height) in inches for a single-column, single-panel plot."""
    return (width, width / ratio)


def figsize_multi_panel(n_panels, width=FIG_WIDTH_DOUBLE_COL, panel_height=1.4):
    """(width, height) for a double-column figure with n_panels stacked
    vertically (e.g. a multi-row timeline) -- panel_height=1.4in keeps each
    row legible without the whole figure exceeding a page (n_panels=6 -> 8.4in,
    already near the practical ceiling for a single-page figure).
    """
    return (width, panel_height * n_panels)


def figsize_row(
    n_subplots,
    width=FIG_WIDTH_DOUBLE_COL,
    height=FIG_WIDTH_DOUBLE_COL / (GOLDEN_RATIO * 1.4),
):
    """(width, height) for n_subplots arranged side-by-side in one row
    (e.g. summary bar charts) -- fixed double-column width, height tuned so
    each subplot stays close to the golden ratio individually.
    """
    return (width, height)


def figsize_grid(n_rows, n_cols, width=FIG_WIDTH_DOUBLE_COL):
    """(width, height) for an n_rows x n_cols subplot grid, each cell close
    to the golden ratio -- height scales with n_rows/n_cols so a 2x2 grid
    stays roughly square-ish rather than needlessly tall or squashed.
    """
    return (width, (width / n_cols) * n_rows / GOLDEN_RATIO * 1.15)


def savefig_academic(fig, out_dir, basename):
    """Saves fig as both <basename>.pdf (vector, primary) and
    <basename>.png (PRINT_DPI raster, fallback for viewers without PDF
    support) -- the single place both formats/DPI are decided, so every
    figure-generating script stays consistent instead of each hardcoding
    its own dpi= value (an earlier version of this repo's scripts used
    dpi=150, below print-quality).
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
