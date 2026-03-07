#!/usr/bin/env bash
#
# benchmark_container_create.sh
#
# Benchmarks container creation latency comparing two image caching strategies:
#
#   1. image-fetcher style: image content fetched but NOT unpacked to snapshotter.
#      Container create must first unpack all layers → slower first-create.
#
#   2. image-pull style: image fully pulled with layers pre-unpacked to snapshotter.
#      Container create uses existing snapshots → fast.
#
# This measures the "unpack penalty" incurred when using image-fetcher during VHD build.
#
# Usage:
#   sudo ./benchmark_container_create.sh [image] [iterations]
#
# Examples:
#   sudo ./benchmark_container_create.sh
#   sudo ./benchmark_container_create.sh mcr.microsoft.com/oss/kubernetes/pause:3.9 20
#
# Environment variables:
#   SNAPSHOTTER   - containerd snapshotter (default: overlayfs)
#   DROP_CACHES   - set to "true" to drop page cache between iterations
#   RESULTS_DIR   - output directory (default: /tmp/container-create-benchmark-<timestamp>)
#
# Prerequisites:
#   - containerd running on the host
#   - Root access (for ctr and optional cache dropping)
#   - python3 + matplotlib + numpy (optional, for graph generation)

set -euo pipefail

readonly IMAGE="${1:-mcr.microsoft.com/azureml/inference-base-cuda12.4-ubuntu22.04:20260302.v1}"
readonly ITERATIONS="${2:-10}"
readonly BENCH_NS="benchmark"
readonly CTR="ctr -n ${BENCH_NS}"
readonly SNAPSHOTTER="${SNAPSHOTTER:-overlayfs}"
readonly DROP_CACHES="${DROP_CACHES:-false}"
readonly RESULTS_DIR="${RESULTS_DIR:-/tmp/container-create-benchmark-$(date +%Y%m%d-%H%M%S)}"
readonly CONTAINER_NAME="bench-ctr"

mkdir -p "${RESULTS_DIR}"
readonly CSV="${RESULTS_DIR}/results.csv"
readonly GRAPH="${RESULTS_DIR}/benchmark.png"

# ──────────────────────────────────────────────
# Helper functions
# ──────────────────────────────────────────────

log() { echo "[$(date +%H:%M:%S)] $*"; }
die() { log "ERROR: $*" >&2; exit 1; }

now_ns() {
    date +%s%N
}

remove_all_snapshots() {
    local pass snaps
    for pass in $(seq 1 15); do
        snaps=$(${CTR} snapshots ls | awk 'NR>1 {print $1}' | tac) || true
        if [[ -z "${snaps}" ]]; then
            return 0
        fi
        while IFS= read -r snap; do
            ${CTR} snapshots rm "${snap}" 2>/dev/null || true
        done <<< "${snaps}"
    done
}

drop_caches_if_enabled() {
    if [[ "${DROP_CACHES}" == "true" ]]; then
        sync
        echo 3 > /proc/sys/vm/drop_caches 2>/dev/null || log "  warning: could not drop caches (not root?)"
    fi
}

cleanup_container() {
    ${CTR} tasks kill "${CONTAINER_NAME}" 2>/dev/null || true
    ${CTR} tasks rm "${CONTAINER_NAME}" 2>/dev/null || true
    ${CTR} containers rm "${CONTAINER_NAME}" 2>/dev/null || true
}

full_cleanup() {
    cleanup_container
    # ${CTR} images rm "${IMAGE}" 2>/dev/null || true
    remove_all_snapshots
}

# Unpack an image's layers into the snapshotter.
# Tries `ctr images unpack` first (containerd 1.x).
# Falls back to `ctr images pull` which re-pulls from cached content and
# unpacks as part of the pull (containerd 2.x removed the standalone unpack command).
unpack_image() {
    # Fallback: re-pull from cached content (essentially just unpacks)
    # ${CTR} images pull "$1" >/dev/null 2>&1
}

# ──────────────────────────────────────────────
# Preflight checks
# ──────────────────────────────────────────────

command -v ctr >/dev/null 2>&1 || die "ctr not found — is containerd installed?"

if ! ctr version >/dev/null 2>&1; then
    die "cannot connect to containerd — is the daemon running? are you root?"
fi

# ──────────────────────────────────────────────
# Banner
# ──────────────────────────────────────────────

cat << EOF

╔══════════════════════════════════════════════════════════════════╗
║           Container Create Latency Benchmark                     ║
╠══════════════════════════════════════════════════════════════════╣
║  Image:       ${IMAGE}
║  Iterations:  ${ITERATIONS}
║  Namespace:   ${BENCH_NS}  (isolated from k8s.io workloads)
║  Snapshotter: ${SNAPSHOTTER}
║  Drop caches: ${DROP_CACHES}
║  Results dir: ${RESULTS_DIR}
╚══════════════════════════════════════════════════════════════════╝

EOF

# ──────────────────────────────────────────────
# Step 1: Ensure image content is cached locally
# ──────────────────────────────────────────────

log "Ensuring image content is cached locally..."
log "(First pull may take a while for large images — subsequent runs reuse cached content)"

if ! ${CTR} images ls -q 2>/dev/null | grep -qF "${IMAGE}"; then
    log "Pulling image into benchmark namespace (one-time download)..."
    ${CTR} images pull "${IMAGE}" 2>&1 | tail -5
else
    log "Image already present in benchmark namespace."
fi

# ──────────────────────────────────────────────
# Step 2: Setup benchmark namespace
# ──────────────────────────────────────────────

log "Setting up benchmark namespace..."
full_cleanup

# Pull into benchmark namespace — content is shared (instant), creates image record + unpacks
# ${CTR} images pull "${IMAGE}" >/dev/null 2>&1
log "Benchmark namespace ready."

# ──────────────────────────────────────────────
# Step 3: Run benchmark
# ──────────────────────────────────────────────

echo "iteration,method,unpack_ms,create_ms,total_ms" > "${CSV}"

echo ""
log "Starting benchmark (${ITERATIONS} iterations)..."
echo ""
printf "%-5s  %-15s  %10s  %10s  %10s\n" "ITER" "METHOD" "UNPACK(ms)" "CREATE(ms)" "TOTAL(ms)"
printf "%-5s  %-15s  %10s  %10s  %10s\n" "----" "------" "----------" "----------" "----------"

for i in $(seq 1 "${ITERATIONS}"); do

    # ─── IMAGE-FETCHER STYLE ─────────────────
    #
    # Simulate: image was cached with image-fetcher during VHD build.
    # Content blobs exist in content store but layers are NOT unpacked
    # to the snapshotter. First container create must pay the unpack cost.
    #
    # We achieve this state by removing all snapshots while keeping the
    # image record and content intact.

    cleanup_container
    remove_all_snapshots
    drop_caches_if_enabled

    # Measure: unpack + container create
    t0=$(now_ns)
    unpack_image "${IMAGE}"
    t1=$(now_ns)
    ${CTR} containers create "${IMAGE}" "${CONTAINER_NAME}" >/dev/null 2>&1
    t2=$(now_ns)

    fetcher_unpack_ms=$(( (t1 - t0) / 1000000 ))
    fetcher_create_ms=$(( (t2 - t1) / 1000000 ))
    fetcher_total_ms=$(( (t2 - t0) / 1000000 ))

    echo "${i},image-fetcher,${fetcher_unpack_ms},${fetcher_create_ms},${fetcher_total_ms}" >> "${CSV}"
    printf "%-5s  %-15s  %10s  %10s  %10s\n" \
        "${i}" "image-fetcher" "${fetcher_unpack_ms}" "${fetcher_create_ms}" "${fetcher_total_ms}"

    # ─── IMAGE-PULL STYLE ────────────────────
    #
    # Simulate: image was cached with full pull (e.g. crictl pull) during VHD build.
    # Content blobs exist AND layers are already unpacked to the snapshotter.
    # Container create only needs to set up the writable layer.
    #
    # After the fetcher test above, the image IS unpacked (we just unpacked it).
    # We only need to remove the container, not the snapshots.

    cleanup_container
    drop_caches_if_enabled

    # Measure: container create only (already unpacked)
    t0=$(now_ns)
    ${CTR} containers create "${IMAGE}" "${CONTAINER_NAME}" >/dev/null 2>&1
    t1=$(now_ns)

    pull_create_ms=$(( (t1 - t0) / 1000000 ))

    echo "${i},image-pull,0,${pull_create_ms},${pull_create_ms}" >> "${CSV}"
    printf "%-5s  %-15s  %10s  %10s  %10s\n" \
        "${i}" "image-pull" "0" "${pull_create_ms}" "${pull_create_ms}"
done

# ──────────────────────────────────────────────
# Final cleanup
# ──────────────────────────────────────────────

full_cleanup

echo ""
log "Benchmark complete."
echo ""
log "Raw CSV results:"
echo ""
column -t -s, "${CSV}" 2>/dev/null || cat "${CSV}"
echo ""
log "CSV saved to: ${CSV}"

# # ──────────────────────────────────────────────
# # Step 4: Generate graph and summary
# # ──────────────────────────────────────────────

# echo ""
# log "Generating graph and summary..."

# export CSV GRAPH

# python3 << 'PYEOF'
# import csv
# import os
# import sys

# csv_path = os.environ["CSV"]
# graph_path = os.environ["GRAPH"]

# # ── Parse CSV ──
# fetcher_unpack, fetcher_create, fetcher_total = [], [], []
# pull_create, pull_total = [], []
# iterations = []

# with open(csv_path) as f:
#     reader = csv.DictReader(f)
#     for row in reader:
#         it = int(row["iteration"])
#         if row["method"] == "image-fetcher":
#             iterations.append(it)
#             fetcher_unpack.append(int(row["unpack_ms"]))
#             fetcher_create.append(int(row["create_ms"]))
#             fetcher_total.append(int(row["total_ms"]))
#         else:
#             pull_create.append(int(row["create_ms"]))
#             pull_total.append(int(row["total_ms"]))

# if not iterations:
#     print("  No data found in CSV.")
#     sys.exit(1)

# # ── Text summary (always printed) ──
# import statistics

# def stats(data):
#     return {
#         "avg": statistics.mean(data),
#         "median": statistics.median(data),
#         "min": min(data),
#         "max": max(data),
#         "stdev": statistics.stdev(data) if len(data) > 1 else 0,
#     }

# fs, ps = stats(fetcher_total), stats(pull_total)
# us = stats(fetcher_unpack)
# penalty = fs["avg"] - ps["avg"]
# speedup = fs["avg"] / max(ps["avg"], 1)

# print()
# print("=" * 65)
# print("  SUMMARY")
# print("=" * 65)
# print(f"  image-fetcher (unpack + create):")
# print(f"    avg={fs['avg']:.0f}ms  median={fs['median']:.0f}ms  "
#       f"min={fs['min']}ms  max={fs['max']}ms  stdev={fs['stdev']:.0f}ms")
# print(f"    └─ unpack:  avg={us['avg']:.0f}ms  median={us['median']:.0f}ms  "
#       f"min={us['min']}ms  max={us['max']}ms")
# print(f"    └─ create:  avg={statistics.mean(fetcher_create):.0f}ms")
# print()
# print(f"  image-pull (create only):")
# print(f"    avg={ps['avg']:.0f}ms  median={ps['median']:.0f}ms  "
#       f"min={ps['min']}ms  max={ps['max']}ms  stdev={ps['stdev']:.0f}ms")
# print()
# print(f"  ⚡ Unpack penalty:  {penalty:.0f}ms  ({speedup:.1f}× slower with image-fetcher)")
# print("=" * 65)
# print()

# # ── Graph (requires matplotlib) ──
# try:
#     import matplotlib
#     matplotlib.use("Agg")
#     import matplotlib.pyplot as plt
#     import numpy as np
# except ImportError:
#     print("  matplotlib/numpy not available — install with:")
#     print("    pip3 install matplotlib numpy")
#     print(f"  CSV results at: {csv_path}")
#     sys.exit(0)

# x = np.array(iterations)
# w = 0.35

# fig, axes = plt.subplots(3, 1, figsize=(14, 16), gridspec_kw={"hspace": 0.35})

# # ── Plot 1: Side-by-side total time bar chart ──
# ax = axes[0]
# b1 = ax.bar(x - w / 2, fetcher_total, w,
#             label="image-fetcher (unpack + create)",
#             color="#e74c3c", alpha=0.85, edgecolor="white", linewidth=0.5)
# b2 = ax.bar(x + w / 2, pull_total, w,
#             label="image-pull (create only)",
#             color="#2ecc71", alpha=0.85, edgecolor="white", linewidth=0.5)
# ax.set_xlabel("Iteration", fontsize=11)
# ax.set_ylabel("Time (ms)", fontsize=11)
# ax.set_title("Container Create Latency: image-fetcher vs image-pull",
#              fontsize=14, fontweight="bold")
# ax.set_xticks(x)
# ax.legend(loc="upper right", fontsize=10)
# ax.grid(axis="y", alpha=0.3, linestyle="--")

# for bars in (b1, b2):
#     for bar in bars:
#         h = bar.get_height()
#         ax.annotate(f"{h:.0f}",
#                     xy=(bar.get_x() + bar.get_width() / 2, h),
#                     xytext=(0, 3), textcoords="offset points",
#                     ha="center", va="bottom", fontsize=7, color="0.3")

# # ── Plot 2: Stacked breakdown ──
# ax = axes[1]
# ax.bar(x - w / 2, fetcher_unpack, w,
#        label="Unpack (fetcher only)", color="#e74c3c", alpha=0.85)
# ax.bar(x - w / 2, fetcher_create, w, bottom=fetcher_unpack,
#        label="Container create (fetcher)", color="#f39c12", alpha=0.85)
# ax.bar(x + w / 2, pull_create, w,
#        label="Container create (pull)", color="#2ecc71", alpha=0.85)
# ax.set_xlabel("Iteration", fontsize=11)
# ax.set_ylabel("Time (ms)", fontsize=11)
# ax.set_title("Time Breakdown: Unpack vs Container Create", fontsize=13)
# ax.set_xticks(x)
# ax.legend(loc="upper right", fontsize=10)
# ax.grid(axis="y", alpha=0.3, linestyle="--")

# # ── Plot 3: Line chart with trend ──
# ax = axes[2]
# ax.plot(x, fetcher_total, "o-", color="#e74c3c", linewidth=2, markersize=6,
#         label="image-fetcher total")
# ax.plot(x, fetcher_unpack, "s--", color="#e67e22", linewidth=1.5, markersize=5,
#         label="image-fetcher unpack")
# ax.plot(x, fetcher_create, "^:", color="#d4ac0d", linewidth=1, markersize=4,
#         label="image-fetcher create")
# ax.plot(x, pull_total, "o-", color="#2ecc71", linewidth=2, markersize=6,
#         label="image-pull total")

# ax.axhline(y=np.mean(fetcher_total), color="#e74c3c", linestyle=":",
#            alpha=0.5, label=f"fetcher avg ({np.mean(fetcher_total):.0f}ms)")
# ax.axhline(y=np.mean(pull_total), color="#2ecc71", linestyle=":",
#            alpha=0.5, label=f"pull avg ({np.mean(pull_total):.0f}ms)")

# ax.set_xlabel("Iteration", fontsize=11)
# ax.set_ylabel("Time (ms)", fontsize=11)
# ax.set_title("Trend Across Iterations", fontsize=13)
# ax.set_xticks(x)
# ax.legend(loc="upper right", fontsize=8, ncol=2)
# ax.grid(alpha=0.3, linestyle="--")

# # ── Summary box at the bottom ──
# summary_text = (
#     f"image-fetcher avg: {np.mean(fetcher_total):.0f}ms "
#     f"(unpack: {np.mean(fetcher_unpack):.0f}ms + create: {np.mean(fetcher_create):.0f}ms)\n"
#     f"image-pull avg: {np.mean(pull_total):.0f}ms\n"
#     f"Unpack penalty: {penalty:.0f}ms  ({speedup:.1f}× slower with image-fetcher)"
# )
# fig.text(0.5, 0.005, summary_text, ha="center", fontsize=11, style="italic",
#          bbox=dict(boxstyle="round,pad=0.6", facecolor="wheat", alpha=0.6))

# plt.savefig(graph_path, dpi=150, bbox_inches="tight")
# print(f"  Graph saved to: {graph_path}")
# PYEOF

# echo ""
# log "All done! Results in ${RESULTS_DIR}/"
# ls -lh "${RESULTS_DIR}/"
