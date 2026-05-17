FROM node:alpine
RUN npm install -g serve
WORKDIR /app
COPY . .
EXPOSE 3000
CMD ["sh", "-c", "sed -i \"s|__API_BASE__|${API_BASE:-https://localhost:8080}|g\" /app/config.js && exec serve . -l 3000"]
