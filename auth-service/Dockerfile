# Build stage
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

RUN mkdir /build

COPY anahita-libs /build/anahita-libs
COPY auth-service /build/auth-service
COPY auth-defaults /build/auth-defaults

WORKDIR /build/anahita-libs
RUN go mod download

WORKDIR /build/auth-service
RUN go mod download

ENV GOPATH=/build
RUN go build -o app-binary ./cmd/api

# Final stage
FROM alpine:3.21
RUN mkdir /app
COPY --from=builder /build/auth-service/app-binary /app
COPY --from=builder /build/auth-service/web /app/web
COPY --from=builder /build/auth-defaults /app/defaults

WORKDIR /app
CMD [ "./app-binary" ]
