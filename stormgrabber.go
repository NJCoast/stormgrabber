package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/mmcdole/gofeed"
)

func main() {

	fp := gofeed.NewParser()
	feed, _ := fp.ParseURL("https://www.nhc.noaa.gov/rss_examples/gis-at.xml")
	fmt.Println(feed.Title)

	for _, item := range feed.Items {
		if strings.Contains(item.Title, "Preliminary Best Track Points [kmz]") {
			fmt.Println(item.Title)
			fmt.Println(item.Link)

			err := DownloadFile("test.kmz", item.Link)
			if err != nil {
				panic(err)
			}
		}
	}

}

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
