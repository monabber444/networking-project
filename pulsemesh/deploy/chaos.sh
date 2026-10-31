#!/usr/bin/env bash
# ============================================================
# PulseMesh Chaos Injection Script
# Injects artificial network degradation using tc/netem to
# simulate real-world WAN conditions on a target container.
#
# Usage:
#   ./deploy/chaos.sh inject  [container] [delay] [jitter] [loss]
#   ./deploy/chaos.sh remove  [container]
#   ./deploy/chaos.sh status  [container]
#
# Defaults target node-3 with 85ms delay, 15ms jitter, 8% loss.
# ============================================================

set -euo pipefail

ACTION="${1:-inject}"
CONTAINER="${2:-pulsemesh-node-3}"
DELAY="${3:-85ms}"
JITTER="${4:-15ms}"
LOSS="${5:-8%}"

case "$ACTION" in
  inject)
    echo "🔥 Injecting chaos on $CONTAINER: delay=${DELAY} jitter=${JITTER} loss=${LOSS}"
    docker exec "$CONTAINER" tc qdisc add dev eth0 root netem delay "$DELAY" "$JITTER" loss "$LOSS"
    echo "✅ Chaos injected. Watch the heatmap at http://localhost:8080"
    ;;
  remove)
    echo "🧹 Removing chaos from $CONTAINER"
    docker exec "$CONTAINER" tc qdisc del dev eth0 root netem 2>/dev/null || true
    echo "✅ Chaos removed."
    ;;
  status)
    echo "📊 Current qdisc on $CONTAINER:"
    docker exec "$CONTAINER" tc qdisc show dev eth0
    ;;
  *)
    echo "Usage: $0 {inject|remove|status} [container] [delay] [jitter] [loss]"
    exit 1
    ;;
esac
