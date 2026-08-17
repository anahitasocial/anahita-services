FROM alpine:latest
RUN mkdir /app

COPY notification-service/app-binary /app

# Run the server executable
CMD [ "/app/app-binary" ]
