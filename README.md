# Stormgrabber

[![GoDoc](https://godoc.org/github.com/NJCoast/stormgrabber?status.svg)](https://godoc.org/github.com/NJCoast/stormgrabber)
[![Go Report Card](https://goreportcard.com/badge/github.com/NJCoast/stormgrabber)](https://goreportcard.com/report/github.com/NJCoast/stormgrabber)

Tool to retrieve National Hurricane Center (NHC) [RSS Feeds](https://www.nhc.noaa.gov/aboutrss.shtml) to get the current atlantic basin hurricane(s) trajectory for NJcoast and generate GeoJSON for input into NJcoast storm surge modeling computational code. Stormgrabber triggers a new calculation when forecast files have been changed by the NHC. The tool parses the rss feed for "Windfield" and "Best Track Points" rss tags for each active storm and then downloads the kmz file specified by the link target. This tool uses [njcoast/kmz2geojson](https://github.com/NJCoast/kmz2geojson) to convert the corresponding kmz to GeoJSON. Stormgrabber will only retreive storms that are in a bounding box surrounding the NJ coast that correspond to the valid spatial extent for NJcoast computational models.
The tool utilizes Amazon AWS S3 buckets for storing the converted files and to give common access to the computational models. Amazon's [AWS SDK for Go](https://aws.amazon.com/sdk-for-go/) is used to provide that functionality. Stormgrabber uses the Go RSS feed library [gofeed](https://github.com/mmcdole/gofeed) to manage parsing of the RSS xml file format. Geospatial 2D geometry support is provided by [orb](https://github.com/paulmach/orb) library.

## Installation

The stormgrabber tool is designed to be run as a [cron](https://en.wikipedia.org/wiki/Cron) and deployed in a containerized environment. This repository contains a [Dockerfile](https://docs.docker.com/engine/reference/builder/) that can be used to build and deploy the tool using docker. To build the container:

```
docker build -t stormgrabber .
```

## Development

Stormgrabber is written in [go](https://golang.org/) for deployment as a microservice. The file stormgrabber.go contains all of the code for the tool. Test driven development is used for development with the main unit tests contained in stormgrabber_test.go. Testing utilizes a [sample NHC rss feed xml file](https://www.nhc.noaa.gov/rss_examples/gis-at.xml) example/gis-at.xml. Code documentation is autogenerated by the [GoDoc](https://godoc.org/) documentation system.

## Limitations

The rss feed tags utilized by the NHC are subject to possible change by the NHC which would result in the tool not downloading the appropriate kmz file. Code saves downloaded and converted files to AWS S3 storage and would have to be modified if moved to a different cloud provider service.
