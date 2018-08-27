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
	"path/filepath"
	"strings"
	"encoding/json"
	"bytes"
	"time"
	"archive/zip"
	
	"github.com/paulmach/go.geojson"
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

	// Get Wind Parameters Field
	wind := make(map[string]int)

	for _, item := range feed.Items {
		if strings.Contains(item.Title, "Advisory Wind Field [shp]") {
			dummy, name := "", ""
			fmt.Sscanf(item.Title, "Advisory Wind Field [shp] - %s Storm %s (%s/%s)", &dummy, &name, &dummy, &dummy)
			name = strings.ToLower(name)

			u, err := url.Parse(item.Link)
			if err != nil {
				log.Fatalln("Failed to parse url for download link item.")
			}

			fullpath := downloadDir + "/" + path.Base(u.RequestURI())

			err = DownloadFile(fullpath, item.Link)
			if err != nil {
				log.Fatalln("Failed to download shp file.")
			}
			log.Println("Downloaded ", fullpath)

			// Open a zip archive for reading.
			r, err := zip.OpenReader(fullpath)
			if err != nil {
					log.Fatal(err)
			}
			
			for _, f := range r.File {
				if strings.Contains(f.Name, "_initialradii"){
					fIn, err := f.Open()
					if err != nil {
						log.Fatal(err)
					}
					defer fIn.Close()

					fOut, err := os.Create(os.TempDir() + "/wind" + filepath.Ext(f.Name))
					if err != nil {
						log.Fatal(err)
					}
					defer fOut.Close()

					if _, err = io.Copy(fOut, fIn); err != nil {
						return
					}
				}
			}
			r.Close()

			// Convert the downloaded file to geojson
			cmd := exec.Command("/usr/bin/ogr2ogr", "-f", "GeoJSON", "-t_srs", "crs:84", "wind.geojson", os.TempDir() + "/wind.shp")

			cmdout, cmderr := cmd.Output()
			if cmderr != nil {
				log.Fatalln(cmderr.Error(), cmdout)
			} else {
				log.Println("Converted shp to geojson for:", fullpath, cmdout)
			}

			// Parse GeoJSON
			fIn, err := os.Open("wind.geojson")
			if err != nil {
				log.Fatalln(err)
			}

			var fc geojson.FeatureCollection
			if err := json.NewDecoder(fIn).Decode(&fc); err != nil {
				log.Fatalln(err)
			}

			for _, feature := range fc.Features {
				wind[name], _ = feature.PropertyInt("RADII")
			}
		}
	}

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
			fullpath := downloadDir + "/" + path.Base(u.RequestURI())

			// Generate geojson file name and path
			geofilepath := downloadDir + "/" + strings.TrimSuffix(path.Base(u.RequestURI()), "kmz") + "geojson"

			err = DownloadFile(fullpath, item.Link)
			if err != nil {
				log.Fatalln("Failed to download kmz file.", item.Link, fullpath)
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

			// Parse GeoJSON
			fIn, err := os.Open(geofilepath)
			if err != nil {
				log.Fatalln(err)
			}

			var fc geojson.FeatureCollection
			if err := json.NewDecoder(fIn).Decode(&fc); err != nil {
				log.Fatalln(err)
			}

			fc.Features[len(fc.Features) - 1].SetProperty("radius", wind[name])

			data, err := json.Marshal(&fc);
			if err != nil {
				log.Fatalln(err)
			}

			log.Println(string(data))

			// Upload geojson file to S3 bucket
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
