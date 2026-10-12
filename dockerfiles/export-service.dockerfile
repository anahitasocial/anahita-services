FROM alpine:latest
RUN mkdir /app

COPY export-service/app-binary /app

# Run the server executable
CMD [ "/app/app-binary" ]
