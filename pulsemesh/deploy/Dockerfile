# ============================================================
# PulseMesh — Multi-stage Docker Build
# Produces a minimal Alpine image (~15 MB) with the compiled
# Go binary and embedded React dashboard.
# ============================================================

# ----------------------------------------------------------
# Stage 1: Build the React web UI
# ----------------------------------------------------------
FROM node:20-alpine AS ui-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# ----------------------------------------------------------
# Stage 2: Build the Go binary (with embedded UI assets)
# ----------------------------------------------------------
FROM golang:alpine AS go-builder
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=ui-builder /app/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /pulsemesh ./cmd/pulsemesh

# ----------------------------------------------------------
# Stage 3: Minimal runtime image
# ----------------------------------------------------------
FROM alpine:3.19
RUN apk add --no-cache iproute2 ca-certificates
COPY --from=go-builder /pulsemesh /usr/local/bin/pulsemesh

# Gossip (SWIM)   — 7946 TCP+UDP
# UDP Probes       — 9000 UDP
# HTTP Dashboard   — 8080 TCP
EXPOSE 7946/udp 7946/tcp 9000/udp 8080/tcp

ENTRYPOINT ["pulsemesh"]
