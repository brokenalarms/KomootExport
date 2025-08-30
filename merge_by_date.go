package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type GPX struct {
	XMLName xml.Name `xml:"gpx"`
	Version string   `xml:"version,attr"`
	Creator string   `xml:"creator,attr"`
	Xmlns   string   `xml:"xmlns,attr"`
	Tracks  []Track  `xml:"trk"`
}

type Track struct {
	XMLName  xml.Name       `xml:"trk"`
	Name     string         `xml:"name"`
	Segments []TrackSegment `xml:"trkseg"`
}

type TrackSegment struct {
	XMLName xml.Name     `xml:"trkseg"`
	Points  []TrackPoint `xml:"trkpt"`
}

type TrackPoint struct {
	XMLName   xml.Name `xml:"trkpt"`
	Latitude  float64  `xml:"lat,attr"`
	Longitude float64  `xml:"lon,attr"`
	Elevation float64  `xml:"ele,omitempty"`
	Time      string   `xml:"time,omitempty"`
	Name      string   `xml:"name,omitempty"`
	Desc      string   `xml:"desc,omitempty"`
}

type RideInfo struct {
	FilePath string
	Name     string
	Date     time.Time
	GPX      GPX
}

func main() {
	var connectGaps bool
	flag.BoolVar(&connectGaps, "connect", false, "Connect gaps between rides with marked segments")
	flag.Parse()

	args := flag.Args()
	if len(args) != 3 {
		fmt.Println("Usage: go run merge_by_date.go [flags] <start_date> <end_date> <output.gpx>")
		fmt.Println("Date format: YYYY-MM-DD")
		fmt.Println("Flags:")
		fmt.Println("  -connect    Connect gaps between rides with marked segments")
		fmt.Println("Example: go run merge_by_date.go -connect 2025-08-15 2025-08-25 merged_rides.gpx")
		os.Exit(1)
	}

	startDateStr := args[0]
	endDateStr := args[1]
	outputFile := args[2]

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		log.Fatalf("Invalid start date format: %v", err)
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		log.Fatalf("Invalid end date format: %v", err)
	}
	endDate = endDate.Add(24 * time.Hour) // Include the end date

	// Find all GPX files
	var gpxFiles []string
	err = filepath.Walk("tours", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(path, ".gpx") {
			gpxFiles = append(gpxFiles, path)
		}
		return nil
	})
	if err != nil {
		log.Fatalf("Error walking tours directory: %v", err)
	}

	var rides []RideInfo

	// Parse each GPX file and extract date
	for _, filePath := range gpxFiles {
		data, err := os.ReadFile(filePath)
		if err != nil {
			log.Printf("Error reading %s: %v", filePath, err)
			continue
		}

		var gpx GPX
		if err := xml.Unmarshal(data, &gpx); err != nil {
			log.Printf("Error parsing %s: %v", filePath, err)
			continue
		}

		// Extract first timestamp to determine ride date
		var rideTime time.Time
		found := false
		for _, track := range gpx.Tracks {
			for _, segment := range track.Segments {
				for _, point := range segment.Points {
					if point.Time != "" {
						rideTime, err = time.Parse("2006-01-02T15:04:05.000Z", point.Time)
						if err != nil {
							log.Printf("Error parsing time %s in %s: %v", point.Time, filePath, err)
							continue
						}
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if found {
				break
			}
		}

		if !found {
			log.Printf("No timestamp found in %s", filePath)
			continue
		}

		// Check if ride is within date range
		if rideTime.After(startDate) && rideTime.Before(endDate) {
			rideName := filepath.Base(filepath.Dir(filePath))
			rides = append(rides, RideInfo{
				FilePath: filePath,
				Name:     rideName,
				Date:     rideTime,
				GPX:      gpx,
			})
			fmt.Printf("Found ride: %s (%s)\n", rideName, rideTime.Format("2006-01-02 15:04"))
		}
	}

	if len(rides) == 0 {
		fmt.Printf("No rides found between %s and %s\n", startDateStr, endDateStr)
		return
	}

	// Sort rides by date
	sort.Slice(rides, func(i, j int) bool {
		return rides[i].Date.Before(rides[j].Date)
	})

	// Create merged GPX with a single track
	mergedGPX := GPX{
		Version: "1.1",
		Creator: "Date Range GPX Merger",
		Xmlns:   "http://www.topografix.com/GPX/1/1",
	}

	// Create a single track
	mergedTrack := Track{
		Name: fmt.Sprintf("Merged rides %s to %s", startDateStr, endDateStr),
	}

	if connectGaps {
		// Single segment with all points, but mark gap connections
		mergedSegment := TrackSegment{}
		
		for i, ride := range rides {
			// Add connection point if not the first ride
			if i > 0 {
				// Get last point of previous ride
				var lastPoint TrackPoint
				if len(mergedSegment.Points) > 0 {
					lastPoint = mergedSegment.Points[len(mergedSegment.Points)-1]
				}
				
				// Get first point of current ride
				var firstPoint TrackPoint
				for _, track := range ride.GPX.Tracks {
					for _, segment := range track.Segments {
						if len(segment.Points) > 0 {
							firstPoint = segment.Points[0]
							break
						}
					}
					if firstPoint.Latitude != 0 {
						break
					}
				}
				
				// Add a waypoint marking the gap
				if lastPoint.Latitude != 0 && firstPoint.Latitude != 0 {
					gapPoint := TrackPoint{
						Latitude:  (lastPoint.Latitude + firstPoint.Latitude) / 2,
						Longitude: (lastPoint.Longitude + firstPoint.Longitude) / 2,
						Name:      "Gap Connection",
						Desc:      fmt.Sprintf("Connected gap between rides"),
					}
					mergedSegment.Points = append(mergedSegment.Points, gapPoint)
				}
			}
			
			// Add all points from current ride
			for _, track := range ride.GPX.Tracks {
				for _, segment := range track.Segments {
					mergedSegment.Points = append(mergedSegment.Points, segment.Points...)
				}
			}
		}
		
		mergedTrack.Segments = []TrackSegment{mergedSegment}
	} else {
		// Multiple segments (preserving breaks between rides)
		for _, ride := range rides {
			for _, track := range ride.GPX.Tracks {
				for _, segment := range track.Segments {
					// Add each segment individually to preserve breaks
					mergedTrack.Segments = append(mergedTrack.Segments, segment)
				}
			}
		}
	}

	mergedGPX.Tracks = []Track{mergedTrack}

	// Write merged GPX
	output, err := xml.MarshalIndent(mergedGPX, "", "  ")
	if err != nil {
		log.Fatalf("Error marshaling GPX: %v", err)
	}

	// Add XML header
	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>` + "\n" + string(output)

	if err := os.WriteFile(outputFile, []byte(xmlContent), 0644); err != nil {
		log.Fatalf("Error writing output file: %v", err)
	}

	fmt.Printf("\nMerged %d rides into %s:\n", len(rides), outputFile)
	for _, ride := range rides {
		fmt.Printf("  - %s (%s)\n", ride.Name, ride.Date.Format("2006-01-02 15:04"))
	}
}