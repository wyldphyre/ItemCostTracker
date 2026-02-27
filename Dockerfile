# Stage 1: Build
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy module file first for layer caching
COPY go.mod ./

# Copy source
COPY . .

# Build a static binary with all assets embedded
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o itemcosttracker .

# Stage 2: Minimal runtime — scratch image (no OS, just the binary)
FROM scratch

COPY --from=builder /app/itemcosttracker /itemcosttracker

# Mount a volume for persistent JSON data
VOLUME ["/data"]

ENV DATA_DIR=/data
ENV ADDR=:8080
EXPOSE 8080

ENTRYPOINT ["/itemcosttracker"]
