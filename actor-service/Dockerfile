# Build stage
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

# Create /build directory
RUN mkdir /build

# Copy libraries and service into /build
COPY anahita-libs /build/anahita-libs
COPY actor-service /build/actor-service

WORKDIR /build/anahita-libs
RUN go mod download

WORKDIR /build/actor-service
RUN go mod download

ENV GOPATH=/build

# Build the service binary
RUN go build -o app-binary ./cmd/api

# Final stage
FROM alpine:3.21

# Set working directory
RUN mkdir /app

# Copy the binary and assets into the final image
COPY --from=builder /build/actor-service/app-binary /app

# Run the server executable
CMD [ "/app/app-binary" ]
