# ---- 阶段 1：构建静态文件 ----
FROM node:22-alpine AS builder

# 国内服务器 npm 加速；能直连 registry.npmjs.org 就删掉这行
ENV NPM_CONFIG_REGISTRY=https://registry.npmmirror.com

WORKDIR /src

COPY package.json package-lock.json ./
RUN npm ci                # 严格按 lock 文件安装：可复现，且命中层缓存

COPY . .
RUN npm run build         # 产物 dist/

# ---- 阶段 2：Nginx 托管静态文件 + 同源反代 API ----
FROM nginx:1.28-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /src/dist /usr/share/nginx/html
EXPOSE 80
