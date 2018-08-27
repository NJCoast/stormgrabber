FROM golang:latest

# Build stormgrabber binary in container
WORKDIR /go/src/github.com/NJCoast/stormgrabber/
RUN go get github.com/mmcdole/gofeed
RUN go get github.com/aws/aws-sdk-go/aws
RUN go get github.com/aws/aws-sdk-go/aws/session
RUN go get github.com/aws/aws-sdk-go/service/s3/s3manager
RUN go get github.com/paulmach/go.geojson

COPY *.go /go/src/github.com/NJCoast/stormgrabber/
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o stormgrabber .

# Deploy Stormgrabber Container
FROM alpine:edge

RUN echo http://nl.alpinelinux.org/alpine/edge/testing >> /etc/apk/repositories \
  && apk add --no-cache python3-dev git gdal proj4-dev build-base libxml2-dev libxslt-dev \
  && pip3 install git+git://github.com/NJCoast/kmz2geojson.git#egg=kmz2geojson awscli lxml

WORKDIR /root/
COPY --from=0 /go/src/github.com/NJCoast/stormgrabber .
CMD ["./stormgrabber"]