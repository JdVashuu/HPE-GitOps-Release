#!/usr/bin/env bash
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$DIR/go-backend"

echo "Starting HPE Recipe Detection Go API..."
exec go run cmd/recipe-api/main.go
