FROM alpine:latest

RUN mkdir -p /app

WORKDIR /app

COPY actor-service/app-binary ./app-binary

RUN chmod +x ./app-binary

CMD [ "./app-binary" ]
