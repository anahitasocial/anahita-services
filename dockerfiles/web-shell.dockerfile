FROM alpine:latest
RUN mkdir -p /app/web

COPY web-shell/app-binary /app

# The web app's build, baked in: `make build_web_shell WEB_BUILD=<folder>`
# copies it to web-shell/web first. docker-compose mounts a folder over it
# instead, so a new build of the app needs no new image.
COPY web-shell/web /app/web

# Run the server executable
CMD [ "/app/app-binary" ]
