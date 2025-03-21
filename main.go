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
	"strings"

	"github.com/carlmjohnson/requests"
	"gopkg.in/yaml.v3"
)

const (
	toursDir  = "tours"
	credsPath = "creds.yaml"
)

func main() {
	collector := NewKomootCollector()
	if err := collector.Login(readCredentials()); err != nil {
		log.Fatalf("Login returned %v", err)
		return
	}
	tours, err := collector.FetchRecordedTours()
	if err != nil {
		log.Fatalf("Fetching tour ids returned %v", err)
		return
	}
	log.Printf("Found %d tours\n", len(tours))
	for _, tour := range tours {
		tourDir := filepath.Join(toursDir, tour.id)
		if _, err := os.Stat(tourDir); err == nil {
			log.Printf("%s already saved\n", tour.id)
			continue
		}
		log.Printf("Downloading tour %s\n", tour.id)
		if err := os.MkdirAll(tourDir, os.ModePerm); err != nil {
			log.Fatalf("Failed to create download directory for tour %s: %v", tour, err)
			return
		}
		if err := collector.DownloadTour(tour, tourDir); err != nil {
			log.Fatalf("Downloading tour %s failed: %v", tour.id, err)
			return
		}
	}
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
}

func (kc *KomootCollector) FetchRecordedTours() ([]*tour, error) {
	var data map[string]any
	if err := requests.URL(fmt.Sprintf("https://www.komoot.com/api/v007/users/%s/tours/", kc.userId)).
		Param("type", "tour_recorded").
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
