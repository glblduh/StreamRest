# Builder
FROM golang:1.18.0-alpine AS builder
WORKDIR /builder
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY streamrest.go ./
RUN go build -o /streamrest

# Deploy
FROM alpine:latest AS deploy
WORKDIR /streamrest
COPY --from=builder /streamrest /streamrest/streamrest
EXPOSE 1010
ENTRYPOINT ["/streamrest/streamrest"]