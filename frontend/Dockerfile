# ── Stage 1: build ────────────────────────────────────────────────────────────
FROM node:22-alpine AS builder

WORKDIR /app

COPY package*.json ./
RUN npm ci

COPY . .

ARG VITE_API_URL=https://api.ravencloak.org
ARG VITE_API_BASE_URL=https://api.ravencloak.org/api/v1
ARG VITE_API_DOMAIN=https://api.ravencloak.org
ARG VITE_LIVEKIT_URL=

RUN npm run build

# ── Stage 2: serve ────────────────────────────────────────────────────────────
FROM nginxinc/nginx-unprivileged:1.27-alpine

COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 8080

CMD ["nginx", "-g", "daemon off;"]
