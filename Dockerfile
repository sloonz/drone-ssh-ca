FROM golang:alpine as builder
RUN apk add --no-cache git ca-certificates
WORKDIR /go/pkg/drone-ssh-ca
COPY . .
RUN go build

FROM alpine:3
EXPOSE 80
ENV GODEBUG netdns=go
COPY --from=builder /go/pkg/drone-ssh-ca/drone-ssh-ca /bin/
ENTRYPOINT ["/bin/drone-ssh-ca"]
