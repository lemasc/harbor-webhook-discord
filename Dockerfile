FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o harbor-discord-webhook .

FROM alpine:3

RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY --from=builder /app/harbor-discord-webhook .

ENV HOST=0.0.0.0
ENV PORT=8080

EXPOSE 8080

ENTRYPOINT ["./harbor-discord-webhook"]
