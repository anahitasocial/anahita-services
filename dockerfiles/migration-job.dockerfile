FROM debian:bookworm-slim

# Install prerequisites
RUN apt-get update && apt-get install -y bash curl ca-certificates default-mysql-client && \
    rm -rf /var/lib/apt/lists/*

# Install Goose
ARG GOOSE_VERSION=3.6.1
RUN curl -L -o /usr/local/bin/goose https://github.com/pressly/goose/releases/download/v${GOOSE_VERSION}/goose_linux_x86_64 && \
    chmod +x /usr/local/bin/goose

# Set working directory
WORKDIR /opt/migration-job

# Copy migration scripts and entrypoint
COPY migration-job /opt/migration-job

# Ensure entrypoint script is executable
RUN chmod +x /opt/migration-job/entrypoint.sh

# Define entrypoint
ENTRYPOINT ["/opt/migration-job/entrypoint.sh"]



