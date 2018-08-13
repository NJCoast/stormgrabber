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
	"encoding/json"
	"bytes"
	"time"
	
	"github.com/mmcdole/gofeed"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

var kmz2geojson = "/usr/bin/kmz2g"

type StormList struct {
	Active []Storm `json:"active_storms"`
}

func (l *StormList) Contains(name string) *Storm {
	for i := 0; i < len(l.Active); i++ {
		if l.Active[i].Name == name {
			return &l.Active[i]
		}
	}
	return nil
}

type Storm struct {
	Name string `json:"name"`
	Type string `json:"type"`
	LastUpdated time.Time `json:"last_updated"`
	Path string `json:"s3_base_path"`
}

func main() {
	var pathslice []string
	var geopathslice []string
	var fullpath string
	var geofilepath string
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

	sess, _ := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1")},
	)
	
	// Download Current Storm Metadata
	resp, err := http.Get("https://s3.amazonaws.com/simulation.njcoast.us/metadata.json")
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	var current, active StormList
	if err := json.NewDecoder(resp.Body).Decode(&current); err != nil{
		log.Fatalln(err)
	}

	// Parse feed from url
	fp := gofeed.NewParser()
	feed, _ := fp.ParseURL(rssurl)
	fmt.Println(feed.Title)

	for _, item := range feed.Items {
		if strings.Contains(item.Title, "Preliminary Best Track Points [kmz]") {
			noActiveStorm = false

			published, err := time.Parse("Mon, 02 Jan 2006 15:04:05 MST", item.Published)
			if err != nil {
				log.Fatalln(err)
			}

			dummy, name := "", ""
			fmt.Sscanf(item.Title, "Preliminary Best Track Points [kmz] - %s Storm %s (%s/%s)", &dummy, &name, &dummy, &dummy)
			name = strings.ToLower(name)

			// Check if we have this storm and need to update it
			storm := current.Contains(name)
			if storm != nil && !published.After(storm.LastUpdated) {
				continue;
			}

			// Update storm parameters
			if storm == nil {
				storm = &Storm{Name: name, Type: "H", LastUpdated: published, Path: fmt.Sprintf("https://s3.amazonaws.com/simulation.njcoast.us/storm/%s/%d/", name, published.Unix())}
			}
			storm.LastUpdated = published
			storm.Path = fmt.Sprintf("https://s3.amazonaws.com/simulation.njcoast.us/storm/%s/%d/", name, published.Unix())

			active.Active = append(active.Active, *storm)

			//Use url parse and path to get the filename
			u, err := url.Parse(item.Link)
			if err != nil {
				log.Fatalln("Failed to parse url for download link item.")
			}
			//Generate filepath
			filename := path.Base(u.RequestURI())
			pathslice = append(pathslice, downloadDir, "/", filename)
			fullpath = strings.Join(pathslice, "")

			// Generate geojson file name and path
			filebase := strings.TrimSuffix(filename, "kmz")
			geopathslice = append(geopathslice, downloadDir, "/", filebase, "geojson")
			geofilepath = strings.Join(geopathslice, "")

			err = DownloadFile(fullpath, item.Link)
			if err != nil {
				log.Fatalln("Failed to download kmz file.")
			}
			log.Println("Downloaded ", fullpath)

			// Convert the downloaded file to geojson
			cmd := exec.Command(kmz2geojson, fullpath, downloadDir)

			cmdout, cmderr := cmd.Output()
			if cmderr != nil {
				log.Fatalln(cmderr.Error(), cmdout)
			} else {
				log.Println("Converted kmz to geojson for:", fullpath, cmdout)
			}

			// Upload geojson file to S3 bucket
			fIn, err := os.Open(geofilepath)
			if err != nil {
				log.Fatalln(err)
			}
			defer fIn.Close()

			_, err = s3manager.NewUploader(sess).Upload(&s3manager.UploadInput{
				ACL:         aws.String("public-read"),
				Bucket:      aws.String("simulation.njcoast.us"),
				Key:         aws.String(fmt.Sprintf("storm/%s/%d/input.geojson", name, published.Unix())),
				ContentType: aws.String("application/json"),
				Body:        fIn,
			})
			if err != nil {
				log.Fatalln(err)
			} else {
				log.Println("Uploaded to S3: ", geofilepath, cmdout)
			}
		}
	}

	// Add any storms that had an update within the last day
	for i := 0; i < len(current.Active); i++ {
		if active.Contains(current.Active[i].Name) == nil && time.Now().Before(current.Active[i].LastUpdated.Add(365 * 24 * time.Hour)) {
			active.Active = append(active.Active, current.Active[i])
		}
	}

	// Upload Metadata
	var buffer bytes.Buffer
	if err := json.NewEncoder(&buffer).Encode(active); err != nil {
		log.Fatalln(err)
	}
	
	_, err = s3manager.NewUploader(sess).Upload(&s3manager.UploadInput{
		ACL:         aws.String("public-read"),
		Bucket:      aws.String("simulation.njcoast.us"),
		Key:         aws.String("metadata.json"),
		ContentType: aws.String("application/json"),
		Body:        &buffer,
	})
	if err != nil {
		log.Fatalln(err)
	}else{
		log.Println("Updated Metadata")
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
