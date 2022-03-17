# Builder
FROM --platform=$BUILDPLATFORM golang:alpine AS builder
RUN apk update && apk add --no-cache git build-base
WORKDIR /builder
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY streamrest.go ./
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-w -s" -o /builder/streamrest

# Deploy
FROM scratch
COPY --from=builder /builder/streamrest /streamrest
EXPOSE 1010
ENTRYPOINT ["/streamrest"]