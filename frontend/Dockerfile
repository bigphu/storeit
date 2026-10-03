# syntax=docker/dockerfile:1

ARG NODE_VERSION=24
ARG NGINX_VERSION=1.30

# --- Dev: Vite với source mount từ host (compose.yml), node_modules trong volume ---
FROM node:${NODE_VERSION}-alpine AS dev
WORKDIR /app
COPY docker/dev.sh /usr/local/bin/dev.sh
EXPOSE 3000
CMD ["sh", "/usr/local/bin/dev.sh"]

# --- Build: type-check + bundle vào /app/dist ---
FROM node:${NODE_VERSION}-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci --no-audit --no-fund
COPY . .
RUN npm run build

# --- Production: nginx (không chạy bằng root) phục vụ SPA, chuyển /api sang app ---
FROM nginxinc/nginx-unprivileged:${NGINX_VERSION}-alpine AS prod
COPY docker/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/ || exit 1
