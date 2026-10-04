FROM node:22.23.2-bookworm-slim AS frontend
WORKDIR /src/webui
COPY webui/package*.json ./
RUN npm ci
COPY webui/ ./
RUN npm run build

FROM golang:1.27.1-alpine3.23 AS backend
ARG VERSION=dev
ARG TARGETOS=linux
ARG TARGETARCH=amd64
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/webui/build ./core/channel/http/webui/dist
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /apigate ./cmd/apigate

FROM alpine:3.23
RUN apk add --no-cache ca-certificates tzdata && addgroup -S apigate && adduser -S -G apigate apigate
WORKDIR /app
COPY --from=backend /apigate /app/apigate
USER apigate
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=3s --start-period=20s CMD wget -q -O /dev/null http://127.0.0.1:8080/ready || exit 1
ENTRYPOINT ["/app/apigate"]
CMD ["serve"]
