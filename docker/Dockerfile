FROM golang:alpine

WORKDIR $GOPATH/src/github.com/mikroskeem/mctrack
COPY . ./
RUN env CGO_ENABLED=0 GOOS=linux go build -a -installsuffix nocgo \
        -ldflags "-s -w" \
        -o /mctrack \
        ./cmd/mctrack

FROM scratch
COPY --from=0 /mctrack /mctrack
COPY --from=0 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

CMD ["/mctrack"]
