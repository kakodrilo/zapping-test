FROM node:alpine
RUN npm install -g serve
WORKDIR /app
COPY . .
EXPOSE 3000
CMD ["serve", ".", "-l", "3000"]
