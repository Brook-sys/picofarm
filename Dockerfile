# syntax=docker/dockerfile:1.7

ARG NODE_VERSION=20
ARG GO_VERSION=1.24
ARG ALPINE_VERSION=3.20

FROM node:${NODE_VERSION}-alpine AS frontend
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
ARG VITE_API_URL=""
ENV VITE_API_URL=${VITE_API_URL}
RUN npm run build

FROM golang:${GO_VERSION}-alpine AS backend
WORKDIR /src
RUN apk add --no-cache ca-certificates git tzdata
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
COPY --from=frontend /src/web/dist ./web/dist
ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown
RUN CGO_ENABLED=0 GOOS=linux go build \
  -trimpath \
  -ldflags "-s -w -X github.com/Brook-sys/picofarm/internal/version.Version=${VERSION} -X github.com/Brook-sys/picofarm/internal/version.Commit=${COMMIT} -X github.com/Brook-sys/picofarm/internal/version.Date=${DATE}" \
  -o /out/picofarm ./cmd/server

FROM alpine:${ALPINE_VERSION} AS runtime
RUN apk add --no-cache ca-certificates tzdata curl && \
  addgroup -S picofarm && \
  adduser -S -D -H -h /app -s /sbin/nologin -G picofarm picofarm
WORKDIR /app
COPY --from=backend /out/picofarm /app/picofarm
COPY --from=frontend /src/web/dist /app/web/dist
COPY migrations /app/migrations
RUN mkdir -p /data/uploads && chown -R picofarm:picofarm /data /app
USER picofarm
ENV ENVIRONMENT=production
ENV PORT=8084
ENV DATABASE_PATH=/data/picofarm.db
ENV UPLOAD_DIR=/data/uploads
EXPOSE 8084
VOLUME ["/data"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD curl -fsS http://localhost:${PORT}/health || exit 1
CMD ["/app/picofarm"]
