# Builder
FROM --platform=$BUILDPLATFORM golang AS builder
WORKDIR /builder
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY streamrest.go ./
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-w -s" -o /builder/streamrest;

# Deploy
FROM gcr.io/distroless/base
COPY --from=builder /builder/streamrest /sr
EXPOSE 1010
ENTRYPOINT ["/sr"] 