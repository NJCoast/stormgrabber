FROM golang:latest

# Build stormgrabber binary in container
WORKDIR /go/src/github.com/NJCoast/stormgrabber/
RUN go get github.com/mmcdole/gofeed

COPY *.go /go/src/github.com/NJCoast/stormgrabber/
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o stormgrabber .

# Deploy Stormgrabber Container
FROM alpine:3.7

RUN apk add --update \
    python \
    py-pip \
    git \
  && pip install git+git://github.com/NJCoast/kmz2geojson.git#egg=kmz2geojson awscli \
  && rm -rf /var/cache/apk/*

# Create data directory for downloads
RUN mkdir /data

WORKDIR /root/
COPY --from=0 /go/src/github.com/NJCoast/stormgrabber .
CMD ["./stormgrabber", "--dir=/data"]