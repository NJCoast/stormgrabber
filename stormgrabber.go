package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/mmcdole/gofeed"
)

var kmz2geojson = "/usr/local/bin/kmz2g"

func main() {
	var pathslice []string
	var fullpath string
	var rssurl string
	var noActiveStorm = true

	wordPtr := flag.String("dir", ".", "KMZ file download directory")
	boolPtr := flag.Bool("test", false, "Use the NHC test RSS Feed")
	flag.Parse()
	downloadDir := *wordPtr

	if *boolPtr {
		rssurl = "https://www.nhc.noaa.gov/rss_examples/gis-at.xml"
	} else {
		rssurl = "https://www.nhc.noaa.gov/gis-at.xml"
	}

	// Parse feed from url
	fp := gofeed.NewParser()
	feed, _ := fp.ParseURL(rssurl)
	fmt.Println(feed.Title)

	for _, item := range feed.Items {
		if strings.Contains(item.Title, "Preliminary Best Track Points [kmz]") {
			noActiveStorm = false
			// fmt.Println(item.Title)
			// fmt.Println(item.Link)
			//Use url parse and path to get the filename
			u, err := url.Parse(item.Link)
			if err != nil {
				panic(err)
			}
			filename := path.Base(u.RequestURI())
			pathslice = append(pathslice, downloadDir, "/", filename)
			fullpath = strings.Join(pathslice, "")
			err = DownloadFile(fullpath, item.Link)
			if err != nil {
				panic(err)
			}
			log.Println("Downloaded ", fullpath)

			// Convert the downloaded file to geojson
			cmd := exec.Command(kmz2geojson, fullpath, downloadDir)

			cmdout, cmderr := cmd.Output()

			if err != nil {
				log.Println(cmderr.Error())
				return
			} else {
				log.Println("Converted kmz to geojson for:", fullpath, cmdout)
			}

		}

	}
	if noActiveStorm {
		log.Println("No Active Storm Download")
	}
}

//DownloadFile is a file downloader utility function
// https://golangcode.com/download-a-file-from-a-url/
func DownloadFile(filepath string, url string) error {

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}
