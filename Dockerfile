FROM golang:1.26-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o new-api-fusion .

FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /build/new-api-fusion /app/new-api-fusion
COPY web /app/web

EXPOSE 3000

ENV TZ=Asia/Shanghai
ENV GIN_MODE=release

ENTRYPOINT ["/app/new-api-fusion"]
