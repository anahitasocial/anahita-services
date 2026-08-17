# Build stage
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates

# Create /build directory
RUN mkdir /build

# Copy libraries and service into /build
COPY anahita-libs /build/anahita-libs
COPY location-service /build/location-service

WORKDIR /build/anahita-libs
RUN go mod download

WORKDIR /build/location-service
RUN go mod download

ENV GOPATH=/build
RUN go build -o app-binary ./cmd/api

# Final stage
FROM alpine:3.21

# Install CA certificates
RUN apk --no-cache add ca-certificates

RUN mkdir /app

# Copy binary
COPY --from=builder /build/location-service/app-binary /app

# Copy CA certificates from builder (extra safety)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

CMD [ "/app/app-binary" ]
