# syntax=docker/dockerfile:1
FROM node:20-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json* ./
RUN npm install
COPY web/ ./
RUN npm run build

FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./cmd/estate/static
RUN CGO_ENABLED=0 go build -o /out/estate ./cmd/estate \
 && CGO_ENABLED=0 go build -o /out/estate-simulator ./cmd/simulator \
 && CGO_ENABLED=0 go build -o /out/estate-agent-gateway ./cmd/agent-gateway

FROM alpine:3.20 AS runtime
RUN apk add --no-cache ca-certificates wget \
 && adduser -D -H estate
WORKDIR /var/lib/estate
COPY --from=build /out/estate /usr/local/bin/estate
COPY --from=build /out/estate-simulator /usr/local/bin/estate-simulator
COPY --from=build /out/estate-agent-gateway /usr/local/bin/estate-agent-gateway
USER estate
EXPOSE 8080
ENV ESTATE_LISTEN=:8080
ENV ESTATE_DATABASE_URL=file:/var/lib/estate/estate.db?_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)
ENTRYPOINT ["/usr/local/bin/estate"]
