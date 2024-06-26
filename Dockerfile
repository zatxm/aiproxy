FROM golang:bookworm
WORKDIR /go/src/anp
ADD . /go/src/anp
ENV CGO_ENABLED=0
RUN go build -ldflags "-s -w" -o main cmd/main.go
RUN cd /go/src/anp \
    && apt update \
    && apt install xz-utils \
    && wget https://github.com/upx/upx/releases/download/v4.2.4/upx-4.2.4-$(go env GOARCH)_linux.tar.xz \
    && tar -xf upx-4.2.4-$(go env GOARCH)_linux.tar.xz \
    && cp upx-4.2.4-$(go env GOARCH)_linux/upx upx \
    && ./upx --best main
FROM gcr.io/distroless/static-debian12
MAINTAINER Tu
WORKDIR /anp
ADD . /anp/data/ssl
ADD . /anp/data/hars
ADD . /anp/data/pics
ADD . /anp/data/cookies
COPY --from=0 /go/src/anp/main /anp/
COPY --from=0 /go/src/anp/etc /anp/data/etc
ENTRYPOINT ["/anp/main", "-c", "/anp/data/etc/c.yaml"]