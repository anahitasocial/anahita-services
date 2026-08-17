# Build stage
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

# Create /build directory
RUN mkdir -p /build

# Copy libraries and service into /build
COPY anahita-libs /build/anahita-libs
COPY group-service /build/group-service

WORKDIR /build/anahita-libs
RUN go mod download

WORKDIR /build/group-service
RUN go mod download

ENV GOPATH=/build

# Build the service binary
RUN go build -o app-binary ./cmd/api

# Final stage
FROM alpine:3.21

# Set working directory
WORKDIR /app

# Copy the binary and assets into the final image
COPY --from=builder /build/group-service/app-binary ./app-binary

# Run the server executable
CMD [ "./app-binary" ]

