FROM alpine:latest
RUN mkdir /app

COPY mail-grpc-service/app-binary /app

# Run the server executable
CMD [ "/app/app-binary" ]
