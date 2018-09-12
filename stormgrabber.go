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
	"regexp"
	"time"
	"archive/zip"
	
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
	"github.com/mmcdole/gofeed"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

var kmz2geojson = "/usr/bin/kmz2g"

type StormList struct {
	Active []*Storm `json:"active_storms"`
}

func (l *StormList) Contains(code string) *Storm {
	for i := 0; i < len(l.Active); i++ {
		if l.Active[i].Code == code {
			return l.Active[i]
		}
	}
	return nil
}

type Storm struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Code string `json:"code"`
	LastUpdated time.Time `json:"last_updated"`
	Path string `json:"s3_base_path"`
}

func ExtractWindTitle(title string) (string, string) {
	result := regexp.MustCompile(`.* Wind Field \[shp\] - .* (\S*)\s\(.*/(\S*)\)`).FindStringSubmatch(title)
	return strings.ToLower(result[1]), strings.ToLower(result[2])
}

func ExtractTrackTitle(title string) (string, string) {
	result := regexp.MustCompile(`.* Best Track Points \[kmz\] - .* (\S*)\s\(.*/(\S*)\)`).FindStringSubmatch(title)
	return strings.ToLower(result[1]), strings.ToLower(result[2])
}

func ExtractBounds(lat, lon string) (*orb.Bound, error) {
	var amin, amax float64
	if _, err := fmt.Sscanf(lat, "%f,%f", &amin, &amax) {
		return nil, err
	}

	var omin, omax float64
	if _, err := fmt.Sscanf(lon, "%f,%f", &omin, &omax) {
		return nil, err
	}

	return &orb.Bound{Min: orb.Point{omin, amin}, Max: orb.Point{omax, amax}}, nil
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Llongfile)
	var noActiveStorm = true

	boolPtr := flag.Bool("test", false, "Use the NHC test RSS Feed")
	awsFolder := flag.String("f", "/", "Folder to upload data to")
	latBound := flag.String("bound-lat", "0.0,90.0", "Bounds of accepted latitude")
	lonBound := flag.String("bound-lon", "-180.0,180.0", "Bounds of accepted latitude")
	flag.Parse()

	boundary, err := ExtractBounds(*latBounds, *lonBound)
	if err != nil {
		log.Fatalln(err)
	}

	var rssurl string
	if *boolPtr {
		rssurl = "https://www.nhc.noaa.gov/rss_examples/gis-at.xml"
	} else {
		rssurl = "https://www.nhc.noaa.gov/gis-at.xml"
	}

	sess, _ := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1")},
	)
	
	// Download Current Storm Metadata
	log.Println("https://s3.amazonaws.com/simulation.njcoast.us/"+*awsFolder+"/metadata.json")
	resp, err := http.Get("https://s3.amazonaws.com/simulation.njcoast.us/"+*awsFolder+"/metadata.json")
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
	wind := make(map[string]float64)

	for _, item := range feed.Items {
		if strings.Contains(item.Title, "Wind Field [shp]") {
			_, code := ExtractWindTitle(item.Title)

			u, err := url.Parse(item.Link)
			if err != nil {
				log.Fatalln("Failed to parse url for download link item.")
			}

			fullpath := os.TempDir() + "/" + path.Base(u.RequestURI())

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
				if strings.Contains(f.Name, "_forecastradii"){
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
				fValue := feature.Properties.MustFloat64("RADII", 0.0)
				if fValue == 0.0 {
					iValue := feature.Properties.MustInt("RADII")
					wind[code] = float64(iValue) * 1.852001
				}else{
					wind[code] = fValue * 1.852001
				}
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

			name, code := ExtractTrackTitle(item.Title)

			// Check if we have this storm and need to update it
			storm := current.Contains(code)
			if storm != nil && !published.After(storm.LastUpdated) {
				continue;
			}

			// Update storm parameters
			if storm == nil {
				storm = &Storm{Name: name, Type: "H", Code: code, LastUpdated: published, Path: fmt.Sprintf("https://s3.amazonaws.com/simulation.njcoast.us/%s/storm/%s/%d/", *awsFolder, code, published.Unix())}
			}
			storm.LastUpdated = published
			storm.Path = fmt.Sprintf("https://s3.amazonaws.com/simulation.njcoast.us/%s/storm/%s/%d/", *awsFolder, code, published.Unix())

			//Use url parse and path to get the filename
			u, err := url.Parse(item.Link)
			if err != nil {
				log.Fatalln("Failed to parse url for download link item.")
			}
			//Generate filepath
			fullpath := os.TempDir() + "/" + path.Base(u.RequestURI())

			// Generate geojson file name and path
			geofilepath := os.TempDir() + "/" + strings.TrimSuffix(path.Base(u.RequestURI()), "kmz") + "geojson"

			err = DownloadFile(fullpath, item.Link)
			if err != nil {
				log.Fatalln("Failed to download kmz file.", item.Link, fullpath)
			}
			log.Println("Downloaded ", fullpath)

			// Convert the downloaded file to geojson
			cmd := exec.Command(kmz2geojson, fullpath, os.TempDir())

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

			fc.Features[len(fc.Features) - 1].Properties["radius"] = wind[code]
			
			inBounds := false
			for _, f := range fc.Features {
				log.Println(f.Geometry.(orb.Point), boundary)
				if boundary.Contains(f.Geometry.(orb.Point)) {
					inBounds = true
				}
			}

			// Ignore boundary for now
			inBounds = true

			if inBounds {
				log.Printf("Storm %s(%s) currently in bounds.\n", name, code)
				active.Active = append(active.Active, storm)
				
				data, err := json.Marshal(&fc);
				if err != nil {
					log.Fatalln(err)
				}

				// Upload geojson file to S3 bucket
				_, err = s3manager.NewUploader(sess).Upload(&s3manager.UploadInput{
					ACL:         aws.String("public-read"),
					Bucket:      aws.String("simulation.njcoast.us"),
					Key:         aws.String(fmt.Sprintf("%s/storm/%s/%d/input.geojson", *awsFolder, code, published.Unix())),
					ContentType: aws.String("application/json"),
					Body:        bytes.NewReader(data),
				})
				if err != nil {
					log.Fatalln(err)
				} else {
					log.Println("Uploaded to S3: ", geofilepath, cmdout)
				}
			}else{
				log.Printf("Storm %s(%s) currently out of bounds.\n", name, code)
			}
		}
	}

	// Add any storms that had an update within the last day
	for i := 0; i < len(current.Active); i++ {
		if active.Contains(current.Active[i].Code) == nil && time.Now().Before(current.Active[i].LastUpdated.Add(365 * 24 * time.Hour)) {
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
		Key:         aws.String(*awsFolder+"/metadata.json"),
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
