FROM alpine:latest
RUN mkdir /app

COPY link-preview-service/app-binary /app

# Run the server executable
CMD [ "/app/app-binary" ]
