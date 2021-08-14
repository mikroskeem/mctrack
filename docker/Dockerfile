FROM golang:alpine AS builder

WORKDIR $GOPATH/src/github.com/mikroskeem/mctrack
COPY . .
RUN env CGO_ENABLED=0 GOOS=linux go build -a -installsuffix nocgo \
        -ldflags "-s -w" \
        -o /mctrack \
        ./cmd/mctrack

FROM scratch
COPY --from=builder /mctrack /mctrack
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

CMD ["/mctrack"]
