FROM alpine:latest

RUN mkdir -p /app

WORKDIR /app

COPY person-service/app-binary ./app-binary

RUN chmod +x ./app-binary

CMD [ "./app-binary" ]
