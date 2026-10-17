FROM alpine:latest

# It reads feeds strangers named, so it runs as nobody in particular.
RUN mkdir /app \
    && adduser -D -H -u 10001 worker

COPY website-service/worker-binary /app

USER worker

CMD [ "/app/worker-binary" ]
