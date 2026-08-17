FROM alpine:latest

# Install CA certificates, update them, and install OpenSSL
RUN apk add --no-cache ca-certificates && update-ca-certificates

RUN mkdir /app

COPY location-service/app-binary /app

# Run the server executable
CMD [ "/app/app-binary" ]
