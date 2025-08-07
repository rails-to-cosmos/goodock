# for glibc 2.31
FROM ubuntu:20.04

ENV DEBIAN_FRONTEND=noninteractive
ENV CGO_ENABLED=1

RUN apt-get update && \
    apt-get install -y wget build-essential && \
    wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz && \
    rm go1.21.0.linux-amd64.tar.gz

ENV PATH="/usr/local/go/bin:${PATH}"

WORKDIR /app

COPY . .

CMD ["go", "build", "-v", "-o", "goodock_linux", "/app"]
