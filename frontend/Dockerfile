# ── Stage 1: build ────────────────────────────────────────────────────────────
FROM node:22-alpine AS builder

WORKDIR /app

COPY package*.json ./
RUN npm ci

COPY . .

ARG VITE_API_URL=https://api.ravencloak.org
ARG VITE_API_BASE_URL=https://api.ravencloak.org/api/v1
ARG VITE_KEYCLOAK_URL=https://auth.ravencloak.org
ARG VITE_KEYCLOAK_REALM=raven
ARG VITE_KEYCLOAK_CLIENT_ID=raven-frontend
ARG VITE_LIVEKIT_URL=

RUN npm run build

# ── Stage 2: serve ────────────────────────────────────────────────────────────
FROM nginx:1.27-alpine

COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
