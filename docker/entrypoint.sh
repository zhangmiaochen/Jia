#!/bin/sh
# 家 · 数字家族档案 —— 容器入口：
#   0) 校验 JWT 签名密钥（未设置 / 仍是开发默认值则拒绝启动）
#   1) 初始化/升级数据库 schema（幂等）
#   2) 后台启动 Nginx（静态文件 + API 反向代理）
#   3) 前台启动 Go API（接收容器停止信号）
set -e

# 登录令牌签名密钥保护：默认值已公开在代码里，绝不用于生产
if [ -z "$JIA_JWT_SECRET" ] || [ "$JIA_JWT_SECRET" = "jia-development-secret-change-me" ]; then
  echo "[jia] 错误：未设置 JIA_JWT_SECRET（或仍为开发默认值），容器拒绝启动！" >&2
  echo "[jia] 请在 docker-compose.yml 同目录创建 .env 并设置该密钥：" >&2
  echo "[jia]   cp .env.example .env" >&2
  echo "[jia]   生成密钥: openssl rand -hex 32" >&2
  exit 1
fi

echo "[jia] 初始化/升级数据库 schema ..."
/app/migrate

echo "[jia] 启动 Nginx ..."
nginx

echo "[jia] 启动 API 服务 (JIA_ADDR=${JIA_ADDR:-:8081}) ..."
exec /app/server
