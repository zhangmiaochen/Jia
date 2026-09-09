# syntax=docker/dockerfile:1
# ============================================================================
# 家 · 数字家族档案 —— 单容器镜像（前端 + 后端一体）
#
# 架构：
#   Node 20/22 阶段  ──> 构建 Vue3 前端 -> dist/
#   golang:1.26     ──> 交叉编译 Go 后端 -> server / migrate（纯 Go SQLite，无 CGO）
#   nginx:alpine    ──> 托管前端静态文件 + 反向代理 /api/v1、/media 到 Go API
#
# 构建与运行：
#   docker build -t jia .
#   docker run -d --name jia \
#     -p 8081:80 \
#     -v jia-data:/data \
#     -e JIA_JWT_SECRET=请替换为随机字符串 \
#     jia
#   然后浏览器访问 http://localhost:8081
#
# 端口说明（重要）：
#   前端打包后默认请求 http://<页面host>:8081/api/v1，因此部署时请保持
#   「宿主机 8081 -> 容器 80」的映射，让页面与 API 同源，同时避开后端仅
#   放行 :5173 开发端口的 CORS 限制，无需改一行代码。
#   若使用自定义 API 地址，可通过构建参数覆盖：
#   docker build --build-arg VITE_API_URL=https://api.example.com/api/v1 -t jia .
# ============================================================================

# ---------- 阶段 1：构建前端（Vue3 + Vite） ----------
FROM node:22-alpine AS web-builder
WORKDIR /app

# 可选：国内 npm 镜像加速，如
#   docker build --build-arg NPM_REGISTRY=https://registry.npmmirror.com -t jia .
ARG NPM_REGISTRY=https://registry.npmjs.org
RUN if [ "$NPM_REGISTRY" != "https://registry.npmjs.org" ]; then npm config set registry "$NPM_REGISTRY"; fi

# 先复制依赖清单，利用 Docker 层缓存，源码变动不触发重新 npm ci
COPY Jia_web/package.json Jia_web/package-lock.json ./
RUN npm ci

# 复制源码并构建（vue-tsc 类型检查 + vite build）
COPY Jia_web/index.html Jia_web/vite.config.ts ./
COPY Jia_web/tsconfig.json Jia_web/tsconfig.app.json Jia_web/tsconfig.node.json ./
COPY Jia_web/src ./src

# 可选：覆盖前端 API 地址（留空时使用默认的 http://<host>:8081/api/v1）
ARG VITE_API_URL=""
ENV VITE_API_URL=$VITE_API_URL
RUN npm run build

# ---------- 阶段 2：构建后端（Go） ----------
FROM golang:1.26-alpine AS api-builder
WORKDIR /src

# 可选：国内 Go 模块镜像加速，如
#   docker build --build-arg GOPROXY=https://goproxy.cn,direct -t jia .
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=$GOPROXY

COPY Jia_api/go.mod Jia_api/go.sum ./
RUN go mod download

COPY Jia_api/cmd ./cmd
COPY Jia_api/migrations ./migrations

# modernc.org/sqlite 为纯 Go 驱动，CGO_ENABLED=0 即可交叉编译
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

# ---------- 阶段 3：运行时（Nginx + API + 前端静态文件） ----------
FROM nginx:alpine
WORKDIR /app

# 数据目录（/data 挂载持久化）。注意：容器启动时入口脚本会自动执行迁移。
ENV JIA_DB_PATH=/data/jia.db \
    JIA_STORAGE_DIR=/data/storage \
    JIA_ADDR=:8081
# JIA_JWT_SECRET 务必在运行时通过 -e 覆盖默认的开发密钥！！！

COPY --from=api-builder /out/server /out/migrate ./
COPY --from=api-builder /src/migrations ./migrations
COPY --from=web-builder /app/dist /usr/share/nginx/html

COPY docker/nginx.conf /etc/nginx/conf.d/default.conf
COPY docker/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh && mkdir -p /data/storage

EXPOSE 80
VOLUME ["/data"]

HEALTHCHECK --interval=30s --timeout=3s --start-period=15s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1/ || exit 1

ENTRYPOINT ["/app/entrypoint.sh"]
