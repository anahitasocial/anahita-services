FROM alpine:latest
RUN mkdir /app

COPY story-service/app-binary /app

# Run the server executable
CMD [ "/app/app-binary" ]
