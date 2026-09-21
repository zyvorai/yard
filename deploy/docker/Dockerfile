# syntax=docker/dockerfile:1
FROM node:20-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json* ./
RUN npm install
COPY web/ ./
RUN npm run build

FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./cmd/yard/static
RUN CGO_ENABLED=0 go build -o /out/yard ./cmd/yard \
 && CGO_ENABLED=0 go build -o /out/yard-simulator ./cmd/simulator \
 && CGO_ENABLED=0 go build -o /out/yard-agent-gateway ./cmd/agent-gateway

FROM alpine:3.20 AS runtime
RUN apk add --no-cache ca-certificates wget \
 && adduser -D -H yard
WORKDIR /var/lib/yard
COPY --from=build /out/yard /usr/local/bin/yard
COPY --from=build /out/yard-simulator /usr/local/bin/yard-simulator
COPY --from=build /out/yard-agent-gateway /usr/local/bin/yard-agent-gateway
USER yard
EXPOSE 8080
ENV YARD_LISTEN=:8080
ENV YARD_DATA_DIR=/var/lib/yard
ENV YARD_DATABASE_URL=file:/var/lib/yard/yard.db?_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)
ENTRYPOINT ["/usr/local/bin/yard"]
