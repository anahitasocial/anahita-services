FROM nginx:latest
EXPOSE 80

# The web app's origin, allowed to call the API with the session cookie.
# Set it to the app's public address on a live site.
ENV WEB_ORIGIN=http://localhost:3000

COPY nginx/nginx.conf /etc/nginx/nginx.conf
COPY nginx/templates /etc/nginx/templates
COPY nginx/index.html /etc/nginx/html/index.html
