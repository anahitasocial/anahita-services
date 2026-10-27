FROM debian:bookworm-slim

# Install prerequisites
RUN apt-get update && apt-get install -y bash curl ca-certificates default-mysql-client && \
    rm -rf /var/lib/apt/lists/*

# Install Goose
# goose for the image's own architecture: BuildKit sets TARGETARCH (amd64 or
# arm64), and goose names the first x86_64.
ARG GOOSE_VERSION=3.6.1
ARG TARGETARCH
RUN arch=$([ "$TARGETARCH" = arm64 ] && echo arm64 || echo x86_64) && \
    curl -fL -o /usr/local/bin/goose https://github.com/pressly/goose/releases/download/v${GOOSE_VERSION}/goose_linux_${arch} && \
    chmod +x /usr/local/bin/goose

# Set working directory
WORKDIR /opt/migration-job

# Copy migration scripts and entrypoint
COPY migration-job /opt/migration-job

# Ensure entrypoint script is executable
RUN chmod +x /opt/migration-job/entrypoint.sh

# Define entrypoint
ENTRYPOINT ["/opt/migration-job/entrypoint.sh"]



