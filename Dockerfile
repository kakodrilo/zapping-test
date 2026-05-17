# Etapa de construcción
FROM golang:1.25-alpine AS builder
WORKDIR /app

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ .
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Etapa de ejecución
FROM alpine:latest
WORKDIR /root/

RUN apk --no-cache add ca-certificates

COPY --from=builder /app/main .
COPY .env .

EXPOSE 8080
CMD ["./main"]
