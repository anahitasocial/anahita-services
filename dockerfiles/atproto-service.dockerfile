FROM alpine:latest
# It reads Bluesky over https, and has to know whose certificates to trust.
RUN apk add --no-cache ca-certificates && mkdir /app

COPY atproto-service/app-binary /app

# Run the server executable
CMD [ "/app/app-binary" ]
