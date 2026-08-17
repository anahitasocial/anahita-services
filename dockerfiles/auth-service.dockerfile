FROM alpine:latest
RUN mkdir /app

COPY auth-service/app-binary /app
COPY auth-service/web /app/web
COPY auth-defaults /app/defaults

WORKDIR /app
CMD [ "./app-binary" ]
