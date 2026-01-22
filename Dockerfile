FROM golang:1.21-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
RUN go get github.com/robfig/cron/v3
COPY . .
RUN go install github.com/swaggo/swag/cmd/swag@latest && \
    swag init -g cmd/server/main.go -o docs || true
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/server

FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
ENV TZ=Asia/Bangkok
WORKDIR /app
COPY --from=builder /app/main .
COPY --from=builder /app/docs ./docs
EXPOSE 3000
CMD ["./main"]
