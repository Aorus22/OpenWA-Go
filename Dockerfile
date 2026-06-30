# OpenWA-Go - Dockerfile
# Multi-stage build: FE dashboard + Go backend

# ===== Stage 1: Dashboard Builder =====
FROM docker.io/node:22-alpine AS dashboard-builder

WORKDIR /app

# Copy frontend source (from OpenWA dashboard)
COPY frontend/package*.json ./
RUN npm ci

COPY frontend/ .
RUN npm run build

# ===== Stage 2: Go Builder =====
FROM golang:1.25-alpine AS go-builder

RUN apk add --no-cache git gcc musl-dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -o /app/server ./cmd/server

# ===== Stage 3: Runtime =====
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata sqlite-libs

WORKDIR /app

COPY --from=go-builder /app/server .
COPY --from=dashboard-builder /app/dist /app/dashboard/dist

EXPOSE 2785

ENV SERVE_DASHBOARD=true

CMD ["./server"]
