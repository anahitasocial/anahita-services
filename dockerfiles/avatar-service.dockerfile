FROM alpine:latest
RUN mkdir /app

COPY avatar-service/app-binary /app

# Run the server executable
CMD [ "/app/app-binary" ]
