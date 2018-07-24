package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/mmcdole/gofeed"
)

func main() {
	var pathslice []string
	var fullpath string

	wordPtr := flag.String("dir", ".", "KMZ file download directory")
	flag.Parse()
	downloadDir := *wordPtr
	fmt.Println(downloadDir)

	fp := gofeed.NewParser()
	feed, _ := fp.ParseURL("https://www.nhc.noaa.gov/rss_examples/gis-at.xml")
	fmt.Println(feed.Title)

	for _, item := range feed.Items {
		if strings.Contains(item.Title, "Preliminary Best Track Points [kmz]") {
			fmt.Println(item.Title)
			fmt.Println(item.Link)
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
		}
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
