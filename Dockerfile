# syntax=docker/dockerfile:1.7
FROM node:24-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY api/openapi.yaml /src/api/openapi.yaml
COPY web ./
RUN npm run generate && npm run build

FROM golang:1.27.4-alpine AS server
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY server ./server
COPY --from=web /src/web/dist ./server/web/dist
ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/pms ./server/cmd/pms

FROM alpine:3.23 AS runtime
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S pms && adduser -S -G pms pms \
    && mkdir -p /data/files && chown -R pms:pms /data
COPY --from=server /out/pms /usr/local/bin/pms
USER pms
EXPOSE 8080
ENTRYPOINT ["pms"]
CMD ["serve"]
