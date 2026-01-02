# Stage 1: Build Frontend
FROM node:20-alpine AS frontend-builder

WORKDIR /app/frontend

# Copy frontend package files
COPY frontend/package*.json ./

# Install dependencies
RUN npm ci

# Copy frontend source code
COPY frontend/ ./

# Build for production
RUN npm run build

# Stage 2: Build Backend
FROM golang:1.21-alpine AS backend-builder

WORKDIR /app/backend

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# Copy backend source code
COPY backend/ ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o vanish-server ./cmd/server

# Stage 3: Production Image
FROM nginx:alpine

# Install ca-certificates and supervisor for running multiple processes
RUN apk --no-cache add ca-certificates supervisor

# Copy nginx configuration
COPY frontend/nginx.conf /etc/nginx/conf.d/default.conf

# Update nginx config to proxy to localhost instead of 'backend' service
RUN sed -i 's|http://backend:8080|http://localhost:8080|g' /etc/nginx/conf.d/default.conf

# Copy frontend built assets
COPY --from=frontend-builder /app/frontend/dist /usr/share/nginx/html

# Copy backend binary
COPY --from=backend-builder /app/backend/vanish-server /usr/local/bin/

# Create supervisor configuration
RUN mkdir -p /etc/supervisor.d
COPY <<EOF /etc/supervisor.d/vanish.ini
[supervisord]
nodaemon=true
user=root
logfile=/var/log/supervisor/supervisord.log
pidfile=/var/run/supervisord.pid

[program:backend]
command=/usr/local/bin/vanish-server
autostart=true
autorestart=true
stdout_logfile=/dev/stdout
stdout_logfile_maxbytes=0
stderr_logfile=/dev/stderr
stderr_logfile_maxbytes=0

[program:nginx]
command=/usr/sbin/nginx -g 'daemon off;'
autostart=true
autorestart=true
stdout_logfile=/dev/stdout
stdout_logfile_maxbytes=0
stderr_logfile=/dev/stderr
stderr_logfile_maxbytes=0
EOF

# Create log directory
RUN mkdir -p /var/log/supervisor

# Expose ports
EXPOSE 80 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://localhost/ || exit 1

# Start supervisor to manage both services
CMD ["/usr/bin/supervisord", "-c", "/etc/supervisord.conf"]
