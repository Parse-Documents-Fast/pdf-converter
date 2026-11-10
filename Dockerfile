# syntax=docker/dockerfile:1

# ---- build stage ----
FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/pdf-converter ./cmd/pdf-converter

# ---- run stage ----
FROM alpine:3.20

RUN apk add --no-cache ca-certificates pandoc typst \
    && adduser -D -u 10001 app

WORKDIR /app
COPY --from=build /out/pdf-converter /usr/local/bin/pdf-converter

USER app

EXPOSE 8000

HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null http://localhost:8000/health || exit 1

ENTRYPOINT ["pdf-converter"]
