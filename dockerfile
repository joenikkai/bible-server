# ---- Build stage ----
FROM golang:1.27-bookworm AS builder

WORKDIR /src
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/bible-server .

# ---- Runtime stage ----
FROM scratch
COPY --from=builder /out/bible-server /bible-server
EXPOSE 8080
ENTRYPOINT ["/bible-server"]