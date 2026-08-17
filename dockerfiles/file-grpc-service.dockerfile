FROM alpine:latest
RUN mkdir /app

COPY file-grpc-service/app-binary /app

# Run the server executable
CMD [ "/app/app-binary" ]
