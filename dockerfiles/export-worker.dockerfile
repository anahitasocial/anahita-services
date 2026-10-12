FROM alpine:latest

# The worker makes each archive as a file of its own before it is stored,
# so it has a folder to work in, and runs as nobody in particular.
RUN mkdir /app /work \
    && adduser -D -H -u 10001 worker \
    && chown worker /work

COPY export-service/worker-binary /app

USER worker
ENV EXPORT_WORK_DIR=/work

CMD [ "/app/worker-binary" ]
