#!/bin/sh
# redeploy.sh — rebuild Docker image and restart the container
set -e

echo "==> Building Docker image..."
docker compose build

echo "==> Restarting container..."
docker compose up -d

echo "==> Done. Logs:"
docker compose logs --tail=20 tablic
