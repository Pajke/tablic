#!/bin/sh
set -e
cd client && npm ci && npm run build
cd ../server
mkdir -p static
cp -r ../client/dist/. static/
go build -o tablic .
echo "Built: server/tablic"
