# Builder
FROM --platform=$BUILDPLATFORM golang AS builder
RUN apt-get update && apt-get install --no-install-recommends -y gcc-aarch64-linux-gnu g++-aarch64-linux-gnu
WORKDIR /builder
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY streamrest.go ./
ARG TARGETOS
ARG TARGETARCH
RUN if [ "$TARGETARCH" = "amd64" ]; then\
    CGO_ENABLED=1 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-w -s" -o /builder/streamrest;\
elif [ "$TARGETARCH" = "arm64" ]; then\
    CGO_ENABLED=1 GOOS=${TARGETOS} GOARCH=${TARGETARCH} CC="aarch64-linux-gnu-gcc" CXX="aarch64-linux-gnu-g++" go build -ldflags="-w -s" -o /builder/streamrest;\
fi

# Deploy
FROM golang
COPY --from=builder /builder/streamrest /sr
EXPOSE 1010
ENTRYPOINT ["/sr"] 