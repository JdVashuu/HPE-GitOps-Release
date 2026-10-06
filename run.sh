#!/usr/bin/env bash
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="$DIR/go-backend"
FRONTEND_DIR="$DIR/frontend"

# Detect JavaScript runtime / package manager (prefer bun, fallback to npm / pnpm / yarn)
if command -v bun >/dev/null 2>&1; then
    PM="bun"
    INSTALL_CMD="bun install"
    DEV_CMD="bun run dev"
elif command -v npm >/dev/null 2>&1; then
    PM="npm"
    INSTALL_CMD="npm install"
    DEV_CMD="npm run dev"
elif command -v pnpm >/dev/null 2>&1; then
    PM="pnpm"
    INSTALL_CMD="pnpm install"
    DEV_CMD="pnpm run dev"
elif command -v yarn >/dev/null 2>&1; then
    PM="yarn"
    INSTALL_CMD="yarn install"
    DEV_CMD="yarn dev"
else
    echo "⚠️  No JavaScript runtime found (bun/npm/pnpm/yarn). Frontend will not start."
    PM=""
fi

# Graceful cleanup on Ctrl+C or script termination
cleanup() {
    echo ""
    echo "🛑 Shutting down backend and frontend..."
    lsof -ti :8081 | xargs kill -9 2>/dev/null || true
    lsof -ti :3000 | xargs kill -9 2>/dev/null || true
    if [ -n "$BACKEND_PID" ]; then
        kill -9 "$BACKEND_PID" 2>/dev/null || true
    fi
    if [ -n "$FRONTEND_PID" ]; then
        kill -9 "$FRONTEND_PID" 2>/dev/null || true
    fi
    echo "✅ All services stopped."
    exit 0
}
trap cleanup SIGINT SIGTERM

# 1. Install frontend dependencies if needed
if [ -n "$PM" ]; then
    if [ ! -d "$FRONTEND_DIR/node_modules" ]; then
        echo "📦 Installing frontend dependencies with $PM..."
        (cd "$FRONTEND_DIR" && $INSTALL_CMD)
    fi
fi

# 2. Start Go Backend (Port 8081)
echo "🚀 Starting Go Backend (http://localhost:8081/api)..."
(cd "$BACKEND_DIR" && go run cmd/recipe-api/main.go) &
BACKEND_PID=$!

# Wait briefly for backend to bind port
sleep 1

# 3. Start React Frontend (Port 3000)
if [ -n "$PM" ]; then
    echo "🌐 Starting React Frontend with $PM (http://localhost:3000)..."
    (cd "$FRONTEND_DIR" && $DEV_CMD) &
    FRONTEND_PID=$!
fi

echo ""
echo "=========================================================="
echo "🎉 HPE Recipe Detection is running!"
echo "👉 Web UI (Visualizer & Manager): http://localhost:3000"
echo "👉 Backend REST & WebSocket API: http://localhost:8081/api"
echo "Press Ctrl+C to stop both services."
echo "=========================================================="
echo ""

# Keep running and wait for background jobs
wait "$BACKEND_PID" "$FRONTEND_PID" 2>/dev/null || true
