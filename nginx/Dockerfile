FROM nginx:alpine
RUN apk add --no-cache curl
COPY nginx/nginx.conf /etc/nginx/nginx.conf
COPY segments /usr/share/nginx/html/segments
