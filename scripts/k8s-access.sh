#!/bin/bash
# HermesX K8s Local Access Script
# 用法: ./scripts/k8s-access.sh [start|stop|status]
# 无需修改 /etc/hosts

# Use kind desktop cluster kubeconfig
KUBECONFIG_PATH="/tmp/kind-desktop-kubeconfig"
if [ ! -f "$KUBECONFIG_PATH" ]; then
  KUBECONFIG_PATH=$(kind get kubeconfig --name desktop 2>/dev/null | head -1)
  if [ ! -s "$KUBECONFIG_PATH" ]; then
    echo "❌ kind cluster 'desktop' kubeconfig not found"
    exit 1
  fi
fi
export KUBECONFIG="$KUBECONFIG_PATH"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PID_DIR="$SCRIPT_DIR/.k8s-access-pids"
mkdir -p "$PID_DIR"

start() {
  echo "🚀 Starting HermesX K8s access proxies..."

  # HermesX API (hermesx namespace)
  kubectl port-forward -n hermesx svc/hermesx 8090:8080 > /dev/null 2>&1 &
  echo $! > "$PID_DIR/hermesx-api.pid"

  # WebUI (hermes namespace)
  kubectl port-forward -n hermes svc/hermes-webui 8091:80 > /dev/null 2>&1 &
  echo $! > "$PID_DIR/webui.pid"

  sleep 2

  # Verify
  HERMESX_OK=$(curl -s http://localhost:8090/health/ready 2>/dev/null)
  WEBUI_OK=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8091/ 2>/dev/null)

  if echo "$HERMESX_OK" | grep -q "ready"; then
    echo "✅ HermesX API:  http://localhost:8090"
  else
    echo "❌ HermesX API failed to start"
  fi

  if [ "$WEBUI_OK" = "200" ]; then
    echo "✅ WebUI:       http://localhost:8091"
  else
    echo "❌ WebUI failed to start"
  fi

  echo ""
  echo "📌 Available endpoints:"
  echo "   HermesX API:  http://localhost:8090"
  echo "   OpenAPI UI:  http://localhost:8090/admin.html"
  echo "   Metrics:     http://localhost:8090/metrics"
  echo "   WebUI:       http://localhost:8091"
  echo ""
  echo "💡 To stop: ./scripts/k8s-access.sh stop"
}

stop() {
  echo "🛑 Stopping proxies..."
  for f in "$PID_DIR"/*.pid; do
    if [ -f "$f" ]; then
      pid=$(cat "$f")
      kill "$pid" 2>/dev/null && echo "   Stopped PID $pid"
      rm -f "$f"
    fi
  done
  echo "Done."
}

status() {
  echo "📊 Proxy Status:"
  for name in hermesx-api webui; do
    pidf="$PID_DIR/${name}.pid"
    if [ -f "$pidf" ]; then
      pid=$(cat "$pidf")
      if kill -0 "$pid" 2>/dev/null; then
        echo "   ✅ $name (PID $pid)"
      else
        echo "   ❌ $name (stale PID $pid)"
        rm -f "$pidf"
      fi
    else
      echo "   ⏸  $name (not running)"
    fi
  done
}

case "${1:-start}" in
  start)   start ;;
  stop)    stop ;;
  status)  status ;;
  *)       echo "Usage: $0 [start|stop|status]" ;;
esac
