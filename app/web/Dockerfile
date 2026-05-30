# =============================================================================
# ai-platform web — multi-stage build
# =============================================================================

# ---- Stage 1: Build Vite SPA ----
FROM node:22-alpine AS builder

WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

# ---- Stage 2: Serve with nginx ----
FROM nginx:1.27-alpine

COPY --from=builder /src/dist /usr/share/nginx/html

# nginx config: serve SPA + proxy API/Gateway
COPY <<'EOF' /etc/nginx/conf.d/default.conf
server {
    listen 80;
    root /usr/share/nginx/html;
    index index.html;

    # SPA fallback
    location / {
        try_files $uri $uri/ /index.html;
    }

    # Proxy API/user requests to API server
    location /api/v1/ {
        proxy_pass http://server:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # Proxy admin requests to admin server
    location /api/admin/ {
        proxy_pass http://server:8082;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    location /v1/ {
        proxy_pass http://server:8081;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;

        # SSE support
        proxy_buffering off;
        proxy_cache off;
        proxy_set_header Connection '';
        chunked_transfer_encoding on;
    }

    # Health
    location /health {
        proxy_pass http://server:8080/health;
    }
}
EOF

EXPOSE 80
