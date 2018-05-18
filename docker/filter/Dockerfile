FROM golang:alpine as builder

ADD . /repo 
RUN cd /repo && go build -o patroneosd *.go 

FROM alpine:3.7

RUN mkdir -p /etc/patroneos

COPY --from=builder /repo/patroneosd /usr/bin/patroneosd
COPY docker/filter/config.json /etc/patroneos/config.json

WORKDIR /etc/patroneos

ENTRYPOINT ["/usr/bin/patroneosd"]
