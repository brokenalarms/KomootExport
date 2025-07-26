package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/carlmjohnson/requests"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	toursDir          string
	credsPath         string
	tourType          string
	includeTitleInDir bool
)

const (
	defaultToursDir  = "tours"
	defaultCredsPath = "creds.yaml"
	defaultTourType  = "tour_recorded"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "komoot-downloader",
		Short: "Download Komoot tours",
		Run: func(cmd *cobra.Command, args []string) {
			collector := NewKomootCollector()
			if err := collector.Login(readCredentials()); err != nil {
				log.Fatalf("Login returned %v", err)
				return
			}
			tours, err := collector.FetchTours(tourType)
			if err != nil {
				log.Fatalf("Fetching tour ids returned %v", err)
				return
			}
			log.Printf("Found %d tours\n", len(tours))
			for _, tour := range tours {
				dirName := tour.id
				if includeTitleInDir && tour.title != "" {
					titleSanitized := sanitizeFilename(tour.title)
					dirName = fmt.Sprintf("%s %s", tour.id, titleSanitized)
				}
				tourDir := filepath.Join(toursDir, dirName)
				if _, err := os.Stat(tourDir); err == nil {
					log.Printf("%s already saved\n", tour.id)
					continue
				}
				log.Printf("Downloading tour %s\n", tour.id)
				if err := os.MkdirAll(tourDir, os.ModePerm); err != nil {
					log.Fatalf("Failed to create download directory for tour %s: %v", tour.id, err)
					return
				}
				if err := collector.DownloadTour(tour, tourDir); err != nil {
					log.Fatalf("Downloading tour %s failed: %v", tour.id, err)
					return
				}
				time.Sleep(2 * time.Second)
			}
		},
	}
	rootCmd.Flags().StringVar(&toursDir, "toursDir", defaultToursDir, "Directory for tour downloads")
	rootCmd.Flags().StringVar(&credsPath, "creds", defaultCredsPath, "Path to credentials file")
	rootCmd.Flags().StringVar(&tourType, "tourType", defaultTourType, "Type of tours: tour_recorded or tour_planned")
	rootCmd.Flags().BoolVar(&includeTitleInDir, "includeTitleInDir", false, "Include tour title in directory name")
	cobra.CheckErr(rootCmd.Execute())
}

type KomootCollector struct {
	client *http.Client
	userId string
}

func NewKomootCollector() KomootCollector {
	return KomootCollector{client: &http.Client{Jar: requests.NewCookieJar()}}
}

func (kc *KomootCollector) Login(creds map[string]string) error {
	kc.userId = creds["user_id"]
	// Parse cookies
	cookiesUrl, err := url.Parse("https://account.komoot.com/actions/transfer?type=signin")
	if err != nil {
		panic(err)
	}
	cookies, err := http.ParseCookie(creds["cookie"])
	if err != nil {
		panic(err)
	}
	kc.client.Jar.SetCookies(cookiesUrl, cookies)
	// Do request for other cookies
	return requests.URL("https://account.komoot.com/actions/transfer?type=signin").
		Client(kc.client).
		Fetch(context.Background())
}

func readCredentials() map[string]string {
	creds := map[string]string{}
	f, err := os.Open(credsPath)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	err = yaml.NewDecoder(f).Decode(&creds)
	if err != nil {
		panic(err)
	}
	return creds
}

type tour struct {
	id             string
	gpx            string
	vectorMapImage string
	coverImages    []string
	title          string
}

func (kc *KomootCollector) FetchTours(tourType string) ([]*tour, error) {
	var data map[string]any
	if err := requests.URL(fmt.Sprintf("https://www.komoot.com/api/v007/users/%s/tours/", kc.userId)).
		Param("type", tourType).
		Param("sort_field", "date").
		Param("sort_direction", "desc").
		Param("status", "private").
		ParamInt("page", 0).
		ParamInt("limit", 10000).
		Client(kc.client).
		ToJSON(&data).
		Fetch(context.Background()); err != nil {
		return nil, err
	}
	items := data["_embedded"].(map[string]any)["tours"].([]any)
	tours := []*tour{}
	for _, item := range items {
		tour := &tour{}
		itemMap := item.(map[string]any)
		tour.id = fmt.Sprintf("%.0f", itemMap["id"].(float64))
		tour.title = itemMap["name"].(string)
		tour.gpx = fmt.Sprintf("https://www.komoot.com/tour/%s/download", tour.id)
		// Vector image
		if vectorMapImageMap, ok := itemMap["vector_map_image"].(map[string]any); ok {
			tour.vectorMapImage = vectorMapImageMap["src"].(string)
		}
		// Cover images
		var coverImageData map[string]any
		err := requests.URL(fmt.Sprintf("https://www.komoot.com/api/v007/tours/%s/cover_images/", tour.id)).
			ParamInt("page", 0).
			ParamInt("limit", 10000).
			Client(kc.client).
			ToJSON(&coverImageData).
			Fetch(context.Background())
		if err == nil {
			coverImageEmbedded := coverImageData["_embedded"]
			if coverImageEmbedded != nil {
				coverImageItems := coverImageEmbedded.(map[string]any)["items"].([]any)
				for _, coverImageItem := range coverImageItems {
					src := coverImageItem.(map[string]any)["src"].(string)
					tour.coverImages = append(tour.coverImages, strings.Split(src, "?")[0])
				}
			}
		}
		tours = append(tours, tour)
	}
	return tours, nil
}

func (kc *KomootCollector) DownloadTour(t *tour, downloadPath string) error {
	var errs []error
	errs = append(errs, requests.URL(t.gpx).
		Client(kc.client).
		ToFile(filepath.Join(downloadPath, "tour.gpx")).
		Fetch(context.Background()))
	if t.vectorMapImage != "" {
		errs = append(errs, requests.URL(t.vectorMapImage).
			Client(kc.client).
			ToFile(filepath.Join(downloadPath, "map.jpg")).
			Fetch(context.Background()))
	}
	for i, img := range t.coverImages {
		errs = append(errs, requests.URL(img).
			Client(kc.client).
			ToFile(filepath.Join(downloadPath, fmt.Sprintf("%d.jpg", i))).
			Fetch(context.Background()))
	}
	return errors.Join(errs...)
}

func sanitizeFilename(name string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9\-_\s]`)
	name = reg.ReplaceAllString(name, "_")
	name = strings.TrimSpace(name)
	return name
}
