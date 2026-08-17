# Build stage
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

# Create /build directory
RUN mkdir /build

# Copy libraries and service into /build
COPY anahita-libs /build/anahita-libs
COPY story-service /build/story-service

WORKDIR /build/anahita-libs
RUN go mod download

WORKDIR /build/story-service
RUN go mod download

ENV GOPATH=/build
RUN go build -o app-binary ./cmd/api

# Final stage
FROM alpine:3.21
RUN mkdir /app
COPY --from=builder /build/story-service/app-binary /app

CMD [ "/app/app-binary" ]
