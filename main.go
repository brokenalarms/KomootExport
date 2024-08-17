package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"

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
	ids, err := collector.FetchRecordedTourIDs()
	if err != nil {
		log.Fatalf("Fetching tour ids returned %v", err)
		return
	}
	log.Printf("Found %d tours\n", len(ids))
	for _, id := range ids {
		gpxPath := fmt.Sprintf("%s/%s.gpx", toursDir, id)
		if _, err := os.Stat(gpxPath); err == nil {
			log.Printf("%s already saved\n", gpxPath)
			continue
		}
		log.Printf("Downloading GPX for tour %s\n", id)
		err := collector.DownloadTourGPX(id, gpxPath)
		if err != nil {
			log.Fatalf("Downloading tour %s failed: %v", id, err)
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
	err := requests.URL("https://account.komoot.com/v1/signin").
		Client(kc.client).
		Method(http.MethodPost).
		BodyForm(url.Values{
			"email":    []string{creds["email"]},
			"password": []string{creds["password"]},
			"reason":   []string{"null"},
		}).
		Fetch(context.Background())
	if err != nil {
		return err
	}
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

func (kc *KomootCollector) FetchRecordedTourIDs() ([]string, error) {
	var data map[string]interface{}
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
	items := data["_embedded"].(map[string]interface{})["tours"].([]interface{})
	ids := []string{}
	for _, item := range items {
		id := item.(map[string]interface{})["id"].(float64)
		ids = append(ids, fmt.Sprintf("%.0f", id))
	}
	return ids, nil
}

func (kc *KomootCollector) DownloadTourGPX(id, gpxPath string) error {
	return requests.URL(fmt.Sprintf("https://www.komoot.com/tour/%s/download", id)).
		Client(kc.client).
		ToFile(gpxPath).
		Fetch(context.Background())
}
