FROM alpine:latest

# ffmpeg and ffprobe do the work. The worker runs them on files strangers
# uploaded, so it runs as nobody in particular, with nowhere to write but
# its own working folder.
RUN apk add --no-cache ffmpeg \
    && mkdir /app /work \
    && adduser -D -H -u 10001 worker \
    && chown worker /work

COPY video-service/worker-binary /app

USER worker
ENV VIDEO_WORK_DIR=/work

CMD [ "/app/worker-binary" ]
