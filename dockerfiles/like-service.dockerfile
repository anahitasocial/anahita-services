FROM alpine:latest
RUN mkdir /app

COPY like-service/app-binary /app

# Run the server executable
CMD [ "/app/app-binary" ]
