FROM alpine:latest
RUN mkdir /app

COPY text-service/app-binary /app

# Run the server executable
CMD [ "/app/app-binary" ]
