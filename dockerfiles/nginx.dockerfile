#nginx/Dockerfile
FROM nginx:latest
EXPOSE 80
COPY ../nginx/nginx.conf /etc/nginx/nginx.conf
COPY ../nginx/index.html /etc/nginx/html/index.html
