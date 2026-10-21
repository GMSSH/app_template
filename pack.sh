#!/usr/bin/env bash
# 一键打包：生成 frontend_dist.tar.gz + backend_dist.tar.gz
# 用法：在项目根目录执行 ./pack.sh
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

echo "==> 1/3 前端构建"
(cd frontend && npm run build)
rm -f frontend_dist.tar.gz
COPYFILE_DISABLE=1 tar -czf frontend_dist.tar.gz -C frontend/dist .
echo "✅ frontend_dist.tar.gz"

echo "==> 2/3 后端构建（linux amd64）"
(cd backend && make linux-amd64)
rm -f backend_dist.tar.gz
COPYFILE_DISABLE=1 tar -czf backend_dist.tar.gz -C backend/dist/linux-amd64 .
echo "✅ backend_dist.tar.gz"

echo "==> 3/3 产物"
ls -lh "$ROOT"/frontend_dist.tar.gz "$ROOT"/backend_dist.tar.gz
