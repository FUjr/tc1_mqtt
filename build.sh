#!/usr/bin/env bash
# 构建并推送 amd64 + arm64 双架构镜像到阿里云镜像仓库
# 用法:
#   ./build.sh            # tag=latest
#   ./build.sh v1.0.0     # 自定义 tag

set -euo pipefail

REGISTRY="registry.cn-hangzhou.aliyuncs.com/fjrcn"
TAG="${1:-latest}"
PLATFORMS="linux/amd64,linux/arm64"
BUILDER="mqtt-mgr-builder"

echo "==> 登录阿里云镜像仓库..."
docker login registry.cn-hangzhou.aliyuncs.com

echo "==> 初始化 buildx builder: ${BUILDER}"
docker buildx inspect "${BUILDER}" &>/dev/null \
  || docker buildx create --name "${BUILDER}" --driver docker-container --bootstrap
docker buildx use "${BUILDER}"

echo "==> 构建并推送 backend  [${PLATFORMS}]"
docker buildx build \
  --platform "${PLATFORMS}" \
  --tag "${REGISTRY}/mqtt-mgr-backend:${TAG}" \
  --push \
  ./backend

echo "==> 构建并推送 nginx(frontend)  [${PLATFORMS}]"
docker buildx build \
  --platform "${PLATFORMS}" \
  --tag "${REGISTRY}/mqtt-mgr-nginx:${TAG}" \
  -f ./frontend/Dockerfile \
  --push \
  .

echo ""
echo "✓ 推送完成"
echo "  ${REGISTRY}/mqtt-mgr-backend:${TAG}"
echo "  ${REGISTRY}/mqtt-mgr-nginx:${TAG}"

echo ""
echo "==> 拉取最新镜像并重启服务..."
docker-compose pull backend nginx
docker-compose up -d backend nginx
echo "✓ 服务已重启"
