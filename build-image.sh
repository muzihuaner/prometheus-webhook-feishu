#!/usr/bin/env bash
# 构建 prometheus-webhook-feishu 镜像：自动递增版本 tag，并让 latest 指向该版本
#
# 用法:
#   ./build-image.sh              # 版本 patch +1（默认）
#   ./build-image.sh minor        # 版本 minor +1，patch 归零
#   ./build-image.sh major        # 版本 major +1，minor/patch 归零
#   PUSH=1 ./build-image.sh       # 构建后推送 <版本> 与 latest 两个 tag
#
# 环境变量:
#   REGISTRY    仓库前缀，默认空（Docker Hub）。如: REGISTRY=registry.example.com/your-team
#   IMAGE_NAME  镜像名，默认 muzihuaner/prometheus-webhook-feishu
#   PLATFORM    构建平台，默认 linux/amd64
#
# 版本号记录在仓库根目录 VERSION 文件（首次运行自动初始化为 0.1.0，不递增）

set -euo pipefail
cd "$(dirname "$0")"

BUMP="${1:-patch}"
case "$BUMP" in
  patch|minor|major) ;;
  *) echo "用法: $0 [patch|minor|major]" >&2; exit 1 ;;
esac

REGISTRY="${REGISTRY:-}"
IMAGE_NAME="${IMAGE_NAME:-muzihuaner/prometheus-webhook-feishu}"
PLATFORM="${PLATFORM:-linux/amd64}"
VERSION_FILE="VERSION"

if [[ ! -f "$VERSION_FILE" ]]; then
  NEW_VERSION="0.1.0"
  echo "$NEW_VERSION" > "$VERSION_FILE"
  echo "==> 首次构建，初始化版本 ${NEW_VERSION}（不递增）"
else
  VERSION="$(tr -d '[:space:]' < "$VERSION_FILE")"
  IFS='.' read -r MAJOR MINOR PATCH <<< "$VERSION"
  case "$BUMP" in
    major) MAJOR=$((MAJOR + 1)); MINOR=0; PATCH=0 ;;
    minor) MINOR=$((MINOR + 1)); PATCH=0 ;;
    patch) PATCH=$((PATCH + 1)) ;;
  esac
  NEW_VERSION="$MAJOR.$MINOR.$PATCH"
  echo "$NEW_VERSION" > "$VERSION_FILE"
  echo "==> 版本 $VERSION → $NEW_VERSION ($BUMP)"
fi

FULL_IMAGE="${REGISTRY:+$REGISTRY/}$IMAGE_NAME"
echo "==> 构建 ${FULL_IMAGE}:${NEW_VERSION} 与 ${FULL_IMAGE}:latest（platform=${PLATFORM}）"

docker build --platform "$PLATFORM" \
  -t "$FULL_IMAGE:$NEW_VERSION" \
  -t "$FULL_IMAGE:latest" \
  .

if [[ "${PUSH:-0}" == "1" ]]; then
  echo "==> 推送 $FULL_IMAGE:$NEW_VERSION 与 latest"
  docker push "$FULL_IMAGE:$NEW_VERSION"
  docker push "$FULL_IMAGE:latest"
else
  echo "==> 构建完成（未推送）。推送: PUSH=1 $0$([ "$BUMP" != patch ] && echo " $BUMP")"
fi
