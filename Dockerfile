FROM golang:1.22-alpine AS builder
WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o /app/main ./cmd/main.go

FROM alpine:3.18
WORKDIR /app

COPY --from=builder /app/main /app/main

EXPOSE 80
ENTRYPOINT ["/app/main"]
