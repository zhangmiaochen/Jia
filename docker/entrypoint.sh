#!/bin/sh
# 家 · 数字家族档案 —— 容器入口：
#   1) 初始化/升级数据库 schema（幂等）
#   2) 后台启动 Nginx（静态文件 + API 反向代理）
#   3) 前台启动 Go API（接收容器停止信号）
set -e

echo "[jia] 初始化/升级数据库 schema ..."
/app/migrate

echo "[jia] 启动 Nginx ..."
nginx

echo "[jia] 启动 API 服务 (JIA_ADDR=${JIA_ADDR:-:8081}) ..."
exec /app/server
