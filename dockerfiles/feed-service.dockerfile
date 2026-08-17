FROM alpine:latest
RUN mkdir /app

COPY feed-service/app-binary /app

# Copy default settings directory
COPY feed-defaults /app/defaults

# Run the server executable
CMD [ "/app/app-binary" ]
