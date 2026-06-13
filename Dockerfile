# Stage 1: Build
# Run the compiler on the native build host (fast); cross-compile to the
# target platform set by `docker build --platform`.
FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS builder

WORKDIR /app

# Copy module file first for layer caching
COPY go.mod ./

# Copy source
COPY . .

# TARGETOS/TARGETARCH are populated by buildkit from --platform.
# Defaults target the amd64 deployment server when --platform is omitted.
ARG TARGETOS=linux
ARG TARGETARCH=amd64

# Build a static binary with all assets embedded
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o itemcosttracker .

# Stage 2: Minimal runtime — scratch image (no OS, just the binary).
# Inherits the target platform, so the produced image is labelled correctly.
FROM scratch

COPY --from=builder /app/itemcosttracker /itemcosttracker

# Mount a volume for persistent JSON data
VOLUME ["/data"]

ENV DATA_DIR=/data
ENV ADDR=:8080
EXPOSE 8080

ENTRYPOINT ["/itemcosttracker"]
