FROM debian:stable-slim

RUN apt-get update && apt-get -uy upgrade
RUN apt-get -y install ca-certificates && update-ca-certificates

ADD coredns /coredns

EXPOSE 53 53/udp
ENTRYPOINT ["/coredns"]
