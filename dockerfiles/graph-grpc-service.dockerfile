FROM alpine:latest
RUN mkdir -p /app /app/defaults

# Copy binary
COPY graph-grpc-service/app-binary /app/app-binary

# Copy default settings directory
COPY graph-grpc-defaults /app/defaults

# Run the server executable
CMD [ "/app/app-binary" ]
