# ── Stage 1: build ────────────────────────────────────────────────────────────
FROM node:22-alpine@sha256:8ea2348b068a9544dae7317b4f3aafcdc032df1647bb7d768a05a5cad1a7683f AS builder

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
FROM nginxinc/nginx-unprivileged:1.27-alpine@sha256:65e3e85dbaed8ba248841d9d58a899b6197106c23cb0ff1a132b7bfe0547e4c0

COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 8080

CMD ["nginx", "-g", "daemon off;"]
