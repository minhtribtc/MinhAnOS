FROM node:22-alpine@sha256:fe4f6d74e20370acb04ad889077d14d0f64a9c8ea6a42bc60cddf1e844699914 AS desktop
WORKDIR /workspace
COPY package.json package-lock.json ./
COPY apps/desktop/package.json apps/desktop/package.json
COPY apps/drive/package.json apps/drive/package.json
COPY packages/ packages/
RUN npm ci
COPY apps/desktop/ apps/desktop/
COPY tests/ tests/
COPY contracts/ contracts/
COPY scripts/ scripts/
RUN npm run build --workspace=minhanos-desktop

FROM golang:1.26-alpine@sha256:a2012b4a1d1f9f7e9f44e0d39b805a901e856b1dd80b4e6ee4338db547050308 AS backend
WORKDIR /src
COPY go.mod go.sum ./
COPY contracts/fixtures/ contracts/fixtures/
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
RUN go test ./... && go vet ./... && CGO_ENABLED=0 go build -trimpath -o /minhanos ./cmd/minhanos

FROM alpine:3.22@sha256:2e0b81d60597f449baa56dc25d355367869d2dca4cd57e866f1e2b52af3eb55f
RUN apk add --no-cache ca-certificates tzdata && addgroup -g 10001 minhanos && adduser -D -u 10001 -G minhanos minhanos && mkdir -p /data && chown 10001:10001 /data
COPY --from=backend /minhanos /app/minhanos
COPY --from=desktop /workspace/apps/desktop/dist /app/web
USER 10001:10001
ENV STATIC_DIR=/app/web TZ=Asia/Bangkok
EXPOSE 8081
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s CMD wget -q -O /dev/null http://127.0.0.1:8081/api/health || exit 1
ENTRYPOINT ["/app/minhanos"]
