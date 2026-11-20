FROM node:26-alpine AS web
WORKDIR /src/web
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
# Node 26 ships without corepack: install the pnpm version pinned in packageManager
RUN npm install --global "$(node -p 'require("./package.json").packageManager')" \
    && pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm run build

FROM golang:1.27-alpine AS server
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist web/dist
RUN CGO_ENABLED=0 go build -tags embed -trimpath \
    -ldflags "-s -w -X github.com/wolf29f/hushtongue/internal/config.Version=${VERSION}" \
    -o /hushtongue ./cmd/server

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=server /hushtongue /hushtongue
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s CMD ["/hushtongue", "healthcheck"]
ENTRYPOINT ["/hushtongue"]
