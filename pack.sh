#!/usr/bin/env bash
# 一键打包：web_dist.tar.gz（前端）+ backend_dist.tar.gz（后端）
# 用法：在项目根目录执行 ./pack.sh
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

echo "==> 1/2 前端（原生 HTML/CSS/JS，无构建）"
rm -f web_dist.tar.gz
COPYFILE_DISABLE=1 tar -czf web_dist.tar.gz --exclude=dev-server.mjs --exclude=.radar.token -C web .
echo "✅ web_dist.tar.gz"

echo "==> 2/2 后端构建（linux amd64）"
(cd backend && VERSION=0.1.0 make linux-amd64)
rm -f backend_dist.tar.gz
COPYFILE_DISABLE=1 tar -czf backend_dist.tar.gz -C backend/dist/linux-amd64 .
echo "✅ backend_dist.tar.gz"

echo "==> 产物清单"
ls -lh "$ROOT"/*_dist.tar.gz
