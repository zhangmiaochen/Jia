# 家 · 数字家族档案

一个**家族协作平台**：一个家族可以维护多份族谱，族谱下的每个人物都有关联的家谱（传记/生平）、照片与人物关系，成员多人协作、共同完善家族档案，并生成可视化的关系图。

支持网页端管理 + 可选移动端（`Jia_mobile` 预留，当前为空），后端为单二进制 Go 服务，数据库为纯 Go 驱动的 SQLite（无需额外安装数据库），可一条命令 Docker 部署。

## 功能特性

- **账号体系**：邮箱注册 / 登录 / 忘记密码重置，JWT 鉴权（24h）
- **家族**：创建多个家族，维护姓氏、籍贯、迁徙史、家训、简介等资料
- **成员协作**：邀请成员（邮箱邀请链接 / 7 天有效）、角色权限分级：
  - `owner` 所有者：家族管理员，可管理成员与删除家族
  - `editor` 编辑者：维护族谱、人物、关系、家谱
  - `viewer` 查看者：只读；若「认领」了某个人物，可编辑自己的资料
- **族谱**：一个家族可有多个族谱，族谱与人物多对多关联，可向族谱增删人物
- **人物**：姓名、性别、生卒、籍贯、职业、传记；支持「认领」——本人注册后绑定到自己的人物档案，获得编辑权
- **人物关系**：父亲/母亲/儿子/女儿/丈夫/妻子/兄弟/姐妹/祖辈/叔舅姑姨等固定身份，支持手写自定义关系；关系自动成对生成（正向 + 反向），同一对人物可在不同家族分别维护
- **家谱（传记书籍）**：以某个人物为根，富文本编辑生平家族资料，支持添加协作者共同编纂
- **照片**：人物照片上传，支持说明、分类、拍摄时间、地点，并可把照片与多个人物关联
- **关系图**：以人物或家族为中心的图谱可视化（前端基于 G6 渲染）
- **动态**：家族操作流水（创建/更新/成员变动等）
- **家族合并**：同一家族被几位家人分别建成多份档案时，用合并码把其中一份整体并入另一份（人物、族谱、家谱、照片、成员一并迁入），重复人物在合并前逐条对照确认

## 技术栈

| 端 | 技术 |
|---|---|
| 后端 `Jia_api` | Go 1.26 · chi 路由 · JWT · bcrypt · SQLite（modernc.org/sqlite 纯 Go 驱动，无 CGO） |
| 前端 `Jia_web` | Vue 3 · TypeScript · Vite 6 · Pinia · Vue Router · Arco Design Vue · @antv/G6 · TipTap 富文本 |
| 移动端 `Jia_mobile` | 预留（规划中，当前为空目录） |
| 部署 | Docker（单容器：Nginx + 前端静态文件 + Go API） |

## 目录结构

```
家/
├── Jia_api/                 # Go 后端
│   ├── cmd/
│   │   ├── server/          # API 服务入口（:8081）
│   │   └── migrate/         # 数据库迁移工具（幂等，可重复执行）
│   ├── migrations/          # SQL schema
│   ├── data/                # SQLite 数据文件（本地开发默认 data/jia.db）
│   ├── storage/             # 上传的照片等媒体文件
│   ├── go.mod / go.sum
│   └── run-server.ps1 / run-migrate.ps1
├── Jia_web/                 # Vue3 前端
│   ├── src/                 # 页面：首页/家族/族谱/人物/家谱/关系图/成员/照片/动态/登录
│   ├── vite.config.ts       # 开发端口 5173
│   └── package.json
├── Jia_mobile/              # 移动端（预留）
├── docker/                  # 容器部署配套
│   ├── nginx.conf           # 静态托管 + /api/v1、/media 反向代理
│   └── entrypoint.sh        # 启动时自动迁移 → 启动 Nginx → 启动 API
├── Dockerfile               # 多阶段构建（前端 + 后端 + 运行时）
├── .dockerignore
├── .gitattributes
└── .github/workflows/       # GitHub Actions 自动构建镜像（ghcr.io）
```

## 快速开始（本地开发）

### 环境要求

- Go 1.26+
- Node.js 18+（推荐 22）

### 1. 启动后端

```powershell
cd Jia_api

# 首次启动前初始化数据库（幂等，可重复执行；默认写入 data\jia.db）
.\run-migrate.ps1        # 或 go run ./cmd/migrate

# 启动 API 服务（监听 :8081）
.\run-server.ps1         # 或 go run ./cmd/server
```

> 注意：服务启动时会校验 `users` 表存在，未迁移会直接报错并提示执行 migrate。

### 2. 启动前端

```powershell
cd Jia_web
npm install
npm run dev      # 开发服务器 http://localhost:5173
```

前端默认请求 `http://<页面host>:8081/api/v1`，后端 CORS 已放行 `:5173` 开发端口，本地联调无需配置。

### 3. 构建产物

```powershell
npm run build    # vue-tsc 类型检查 + vite build，产出 Jia_web/dist/
```

### 环境变量（后端）

| 变量 | 默认值 | 说明 |
|---|---|---|
| `JIA_ADDR` | `:8081` | API 监听地址 |
| `JIA_DB_PATH` | `data/jia.db` | SQLite 数据文件路径（相对工作目录解析） |
| `JIA_STORAGE_DIR` | `storage` | 上传媒体目录 |
| `JIA_JWT_SECRET` | `jia-development-secret-change-me` | JWT 签名密钥，**生产环境必须覆盖** |

> 开发环境的「忘记密码」接口会直接把 `reset_token` 返回在响应体中，便于调试；生产部署请自行改造为邮件发送（或在高安全要求下禁用）。

## Docker 部署（前后端一体）

无需预先构建、无需安装 Go/Node，一条命令把前端与后端打包进一个容器：

```bash
# 构建（可选 --build-arg 加速）
docker build -t jia .

# 运行：API 数据与照片持久化到 jia-data 卷
docker run -d --name jia \
  -p 8081:80 \
  -v jia-data:/data \
  -e JIA_JWT_SECRET=请替换为随机字符串 \
  jia

# 访问 http://localhost:8081
```

容器启动时**自动执行数据库迁移**（幂等），无需手工初始化。

### ⚠️ 端口映射必须是 `8081:80`

前端打包后默认请求 `http://<页面host>:8081/api/v1`（见 `Jia_web/src/api/client.ts`），因此把容器 **80** 口映射到宿主机 **8081**，页面与 API 同源，且不触发后端「仅放行 :5173」的 CORS 限制——**代码零改动**。

### 构建参数（可选）

| 参数 | 默认值 | 说明 |
|---|---|---|
| `NPM_REGISTRY` | `https://registry.npmjs.org` | 国内可传 `https://registry.npmmirror.com` |
| `GOPROXY` | `https://proxy.golang.org,direct` | 国内可传 `https://goproxy.cn,direct` |
| `VITE_API_URL` | 空（使用默认同源地址） | 自定义前端 API 地址，如 `https://api.example.com/api/v1`（此时端口映射无需受 8081 约束） |

```bash
docker build --build-arg NPM_REGISTRY=https://registry.npmmirror.com \
             --build-arg GOPROXY=https://goproxy.cn,direct \
             -t jia .
```

## 自动构建（GitHub Actions）

仓库已配置 `.github/workflows/docker.yml`，推送到 GitHub 后自动构建并推送镜像到 GitHub Container Registry（ghcr.io）：

| 触发时机 | 推送的标签 |
|---|---|
| push `main` | `ghcr.io/zhangmiaochen/jia:main` / `:latest` / `:sha-xxxxxx` |
| push `v*` 版本标签 | `ghcr.io/zhangmiaochen/jia:v1.2.3` / `:latest` |
| PR 到 `main` | 只构建验证，不推送（`pr-N` / `sha-xxxxxx`） |

```bash
# 拉取运行（无需本地构建）
docker pull ghcr.io/zhangmiaochen/jia:latest
docker run -d --name jia \
  -p 8081:80 -v jia-data:/data \
  -e JIA_JWT_SECRET=请替换为随机字符串 \
  ghcr.io/zhangmiaochen/jia:latest
```

- 构建使用 BuildKit 层缓存；多架构 `linux/amd64` + `linux/arm64`（arm64 由 QEMU 模拟，速度较慢，只需 x86 时可删去）
- GHCR 包**默认私有**：本地使用需先 `docker login ghcr.io`；想让所有人可拉取，到仓库 Settings → Packages 里把包设为 Public
- GitHub Runner 直连官方 `npmjs` / `proxy.golang.org` 没有问题；如遇网络问题，可在 workflow 的 `build-push-action` 步骤里加 `build-args` 传国内镜像：`NPM_REGISTRY=https://registry.npmmirror.com`、`GOPROXY=https://goproxy.cn,direct`

## 客户端部署（Docker Compose）

把 `docker-compose.yml` 与 `.env.example` 一并交给客户端即可一键部署：

```bash
# 客户端机器上（docker compose v2）
cp .env.example .env          # Windows: copy .env.example .env
# 编辑 .env 设置 JIA_JWT_SECRET（openssl rand -hex 32 生成），每个客户端必须不同
docker compose up -d
# 访问 http://<服务器IP>:8081
```

- 数据持久化在命名卷 `jia-data`（SQLite 数据库 + 上传照片），`docker compose up` 不丢数据；升级用 `docker compose pull && docker compose up -d`
- 忘记设置 `JIA_JWT_SECRET`（或仍用开发默认值）时，容器启动会**校验失败并给出中文提示**，防止误用开发密钥
- 镜像从 `ghcr.io/zhangmiaochen/jia` 拉取：包需设为 Public，或在客户端先 `docker login ghcr.io`（PAT 取 read:packages 权限）
- 支持换成 `build:` 本地源码构建（见 compose 内注释）

### 数据与备份

- 数据库：`/data/jia.db`（SQLite）
- 上传文件：`/data/storage/`
- 备份 = 备份 `/data` 目录（或挂载的 `jia-data` 卷）即可；迁移工具幂等，恢复旧库后启动容器会自动补齐新 schema。

## 家族合并（把多份重复档案合成一份）

同一个家族被几位家人分别注册账号、各自建了「家族」并录了人物和家谱时，不必重新录入——可以把其中一份整体并入另一份。

**流程（双方 owner 共同确认）**

1. 主家族（要保留的那份）的 owner 进入「家族资料 → 家族合并 → 开始合并 → 生成合并码」，把合并码（形如 `7F3K-92QD-XY4M`，7 天有效、只能用一次）发给对方；
2. 被并入家族的 owner 在自己账号里进入「家族资料 → 家族合并 → 输入合并码」，粘贴合并码、选择要并入的家族（他在该家族里必须是 owner）；
3. 系统给出**人物对照**：按「姓名 + 出生日期」建议配对（同名同生日直接建议合并），逐条确认「合并到接收方档案」还是「保留为独立人物」，并可选是否把两边同名的族谱合并成一份；
4. 确认执行后，服务端在**一个事务**内完成迁移并返回统计（人物 / 关系 / 族谱 / 家谱 / 照片 / 成员）。

**迁移规则**

| 数据 | 处理方式 |
|---|---|
| 成员 | 全部加入主家族；被并入方的 `owner` 保留 `owner` 权限。被并入家族不再出现在任何人的家族列表里 |
| 人物 | 未配对的直接迁入；配对的把旧档案并入新档案（空字段补齐，性别、认领关系补全） |
| 人物关系 | 改指到合并后的档案；自环与完全重复的关系自动去掉，正反两行重新配对 |
| 族谱 | 迁入主家族；同名族谱可选合并为一份（人物归属自动并集去重） |
| 家谱（传记） | 迁入主家族，根人物跟随人物合并改指 |
| 照片 | 迁入主家族，人物归属跟随人物合并改指 |
| 动态 | 迁入主家族，最后追加一条「家族合并」记录 |

**注意事项**

- **必须双方 owner**：合并码由接收方 owner 生成，只有被并入方的 owner 才能确认；任何一方都无法单方面搬走对方的数据；
- **没有一键撤销**：执行前请先备份 `Jia_api/data/jia.db`（SQLite 单文件，或 Docker 的 `jia-data` 卷）。合并记录保存在 `family_merges` / `family_merge_pairs` 表里，可追溯「哪个人物并进了哪个人物」；
- 被并入的家族行保留为**无成员的空壳**（内容已全部迁走），不再出现在界面中；
- 合并表由服务启动时自动创建（`CREATE TABLE IF NOT EXISTS`，纯新增），老库无需先跑 migrate 即可使用；
- 自测：`cd Jia_api && go test ./cmd/server -run TestFamilyMerge -v`（端到端跑通合并并用 SQL 校验数据一致性）。

## API 概览

全部路由前缀 `/api/v1`，除注册/登录/找回密码外均需 `Authorization: Bearer <token>`：

| 模块 | 主要路由 |
|---|---|
| 认证 | `POST /auth/register` `/auth/login` `/auth/forgot-password` `/auth/reset-password`，`GET /auth/me` |
| 家族 | `GET|POST /families`、`GET|PATCH|DELETE /families/{id}`、成员 `.../members`、邀请 `POST /families/{id}/invites`、`POST /invites/{token}/accept` |
| 族谱 | `.../genealogies` 增删改查，`.../genealogies/{id}/persons` 关联人物 |
| 人物 | `.../persons` 增删改查（按家族），`POST /persons/{id}/claim` 认领，`POST /persons/{id}/move` 迁移，多家族关联 `.../families` |
| 关系 | `POST /persons/{id}/relations`、`PATCH|DELETE /relations/{id}` |
| 关系图 | `GET /persons/{id}/graph`、`GET /families/{id}/graph` |
| 家谱 | `.../books` 增删改查、`POST /books/{id}/collaborators` 协作者 |
| 照片 | `POST /persons/{id}/photos` 上传、`.../photos` 增删改查、照片-人物关联 |
| 动态 | `GET /families/{id}/activities` |
| 家族合并 | `POST /families/{id}/merge-invites` 生成合并码、`GET /merge-invites/{token}` 查询指向、`POST /merge-invites/{token}/preview` 预览（含人物配对建议）、`POST /merge-invites/{token}/execute` 执行 |

## 常见问题

- **`Access-Control-Allow-Origin` 报错**：后端仅放行 `:5173` 起源。本地开发从 5173 访问没问题；Docker 部署请把容器 80 映射到宿主 8081（同源请求，不走 CORS）；自定义域名用 `VITE_API_URL` 构建参数 + Nginx 同域反代。
- **宿主机 8081 被占用**：把 `docker run -p 8081:80` 改成其他宿主机端口时，前端会去请求页面 host 的 8081，会连不上——请优先释放 8081，或使用 `VITE_API_URL` 指向实际 API 地址。
- **迁移报错「缺少 users 表」**：先运行 `go run ./cmd/migrate`（或进入容器执行 `/app/migrate`）。
- **照片 404**：确认容器 `JIA_STORAGE_DIR` 指向的目录存在且有写入权限（Dockerfile 默认 `/data/storage`，已自动创建）。
- **生产环境务必**：设置强随机 `JIA_JWT_SECRET`；通过反代（如宿主机 Nginx/Caddy）为站点加上 HTTPS。

## 路线图

- [ ] `Jia_mobile` 移动端 App
- [ ] 回收站 / 软删除
- [ ] 忘记密码邮件发送
- [ ] 家谱导出（PDF/图片）
