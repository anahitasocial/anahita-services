FROM alpine:latest
RUN mkdir /app

COPY abuse-report-service/app-binary /app

# Run the server executable
CMD [ "/app/app-binary" ]
