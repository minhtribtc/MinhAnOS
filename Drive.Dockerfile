FROM node:22-alpine@sha256:fe4f6d74e20370acb04ad889077d14d0f64a9c8ea6a42bc60cddf1e844699914 AS web
WORKDIR /workspace
COPY package.json package-lock.json ./
COPY apps/desktop/package.json apps/desktop/package.json
COPY apps/drive/package.json apps/drive/package.json
COPY packages/ packages/
RUN npm ci
COPY apps/drive/ apps/drive/
COPY tests/ tests/
COPY contracts/ contracts/
COPY scripts/ scripts/
RUN npm run build --workspace=minhanos-files

FROM golang:1.26-alpine@sha256:a2012b4a1d1f9f7e9f44e0d39b805a901e856b1dd80b4e6ee4338db547050308 AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY contracts/fixtures/ contracts/fixtures/
RUN go mod download
COPY internal/drive/ internal/drive/
COPY internal/authn/ internal/authn/
COPY internal/coreprofile/ internal/coreprofile/
COPY internal/migrate/ internal/migrate/
COPY internal/identity/ internal/identity/
COPY cmd/identity-admin/ cmd/identity-admin/
COPY cmd/identity-recovery/ cmd/identity-recovery/
COPY internal/apicontract/ internal/apicontract/
COPY cmd/drive/ cmd/drive/
COPY cmd/files-agent/ cmd/files-agent/
RUN go test ./... && go vet ./... && CGO_ENABLED=0 go build -trimpath -o /drive ./cmd/drive && CGO_ENABLED=0 go build -trimpath -o /files-agent ./cmd/files-agent && CGO_ENABLED=0 go build -trimpath -o /identity-admin ./cmd/identity-admin && CGO_ENABLED=0 go build -trimpath -o /identity-recovery ./cmd/identity-recovery
FROM alpine:3.22@sha256:2e0b81d60597f449baa56dc25d355367869d2dca4cd57e866f1e2b52af3eb55f
RUN apk add --no-cache ca-certificates tzdata && addgroup -g 10001 drive && adduser -D -u 10001 -G drive drive && mkdir /data && chown 10001:10001 /data
COPY --from=build /drive /app/drive
COPY --from=build /files-agent /app/files-agent
COPY --from=build /identity-admin /app/identity-admin
COPY --from=build /identity-recovery /app/identity-recovery
COPY --from=web /workspace/apps/drive/dist /app/web
USER 10001:10001
ENV TZ=Asia/Bangkok
EXPOSE 8082
HEALTHCHECK --interval=15s --timeout=3s --start-period=15s CMD wget -q -O /dev/null http://127.0.0.1:8082/api/health || exit 1
ENTRYPOINT ["/app/drive"]
