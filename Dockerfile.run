FROM alpine:3.2
MAINTAINER Jan Broer <janeczku@yahoo.com>

ADD https://github.com/janeczku/go-dnsmasq/releases/download/1.0.0/go-dnsmasq_linux-amd64 /go-dnsmasq
RUN chmod +x /go-dnsmasq

EXPOSE 53 53/udp
ENTRYPOINT ["/go-dnsmasq"]
