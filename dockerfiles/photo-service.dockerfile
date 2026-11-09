FROM alpine:latest
# heif-convert reads a HEIC or an AVIF picture (roadmap row 135).
RUN apk add --no-cache libheif-tools && mkdir /app

COPY photo-service/app-binary /app

# Run the server executable
CMD [ "/app/app-binary" ]
