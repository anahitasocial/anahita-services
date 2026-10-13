FROM alpine:latest
# It fetches from other servers over https, and has to know whose
# certificates to trust.
RUN apk add --no-cache ca-certificates && mkdir /app

COPY activitypub-service/app-binary /app

# Run the server executable
CMD [ "/app/app-binary" ]
