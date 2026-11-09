FROM alpine:latest
# It fetches from other servers over https, and has to know whose
# certificates to trust.
# heif-convert reads a HEIC or an AVIF picture from elsewhere (roadmap
# row 135).
RUN apk add --no-cache ca-certificates libheif-tools && mkdir /app

COPY activitypub-service/app-binary /app

# Run the server executable
CMD [ "/app/app-binary" ]
