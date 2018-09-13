FROM golang:1.9.5 as builder

ENV GOOS linux
ENV GOARCH amd64
ENV CGO_ENABLED 0
RUN go get github.com/yangyuqian/pilot-debug/samples/service

FROM alpine:3.5

WORKDIR /root/
COPY --from=builder /go/bin/service /root/mockserver
COPY --from=builder /go/src/github.com/yangyuqian/pilot-debug/samples/service/server.crt /root/server.crt
COPY --from=builder /go/src/github.com/yangyuqian/pilot-debug/samples/service/server.key /root/server.key

