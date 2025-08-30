package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

// GPX structures
type GpxRoot struct {
	XMLName xml.Name   `xml:"gpx"`
	Tracks  []GpxTrack `xml:"trk"`
}

type GpxTrack struct {
	Name     string       `xml:"name"`
	Segments []GpxSegment `xml:"trkseg"`
}

type GpxSegment struct {
	Points []GpxPoint `xml:"trkpt"`
}

type GpxPoint struct {
	Latitude  float64 `xml:"lat,attr"`
	Longitude float64 `xml:"lon,attr"`
	Elevation float64 `xml:"ele,omitempty"`
	Time      string  `xml:"time,omitempty"`
	Name      string  `xml:"name,omitempty"`
	Desc      string  `xml:"desc,omitempty"`
}

// KML structures
type KML struct {
	XMLName  xml.Name `xml:"kml"`
	Xmlns    string   `xml:"xmlns,attr"`
	Document Document `xml:"Document"`
}

type Document struct {
	Name        string       `xml:"name"`
	Description string       `xml:"description,omitempty"`
	Styles      []Style      `xml:"Style"`
	Placemarks  []Placemark  `xml:"Placemark"`
}

type Style struct {
	ID        string    `xml:"id,attr"`
	LineStyle LineStyle `xml:"LineStyle"`
}

type LineStyle struct {
	Color string  `xml:"color"`
	Width float64 `xml:"width"`
}

type Placemark struct {
	Name        string      `xml:"name"`
	Description string      `xml:"description,omitempty"`
	StyleURL    string      `xml:"styleUrl,omitempty"`
	LineString  *LineString `xml:"LineString,omitempty"`
	Point       *Point      `xml:"Point,omitempty"`
}

type LineString struct {
	Tessellate   int    `xml:"tessellate"`
	AltitudeMode string `xml:"altitudeMode,omitempty"`
	Coordinates  string `xml:"coordinates"`
}

type Point struct {
	Coordinates string `xml:"coordinates"`
}

func main() {
	var (
		lineColor    string
		gapColor     string
		lineWidth    float64
		showWaypoints bool
	)
	
	flag.StringVar(&lineColor, "color", "ff0000ff", "Line color in KML format (aabbggrr, default: red)")
	flag.StringVar(&gapColor, "gap-color", "7f00ffff", "Gap connection color in KML format (aabbggrr, default: yellow)")
	flag.Float64Var(&lineWidth, "width", 4.0, "Line width")
	flag.BoolVar(&showWaypoints, "waypoints", false, "Show waypoints for gap connections")
	flag.Parse()

	args := flag.Args()
	if len(args) != 2 {
		fmt.Println("Usage: go run gpx2kml.go [flags] <input.gpx> <output.kml>")
		fmt.Println("Flags:")
		fmt.Println("  -color string      Line color in KML format (default: ff0000ff)")
		fmt.Println("  -gap-color string  Gap connection color (default: 7f00ffff)")
		fmt.Println("  -width float       Line width (default: 4.0)")
		fmt.Println("  -waypoints         Show waypoints for gap connections")
		os.Exit(1)
	}

	inputFile := args[0]
	outputFile := args[1]

	// Read GPX file
	data, err := os.ReadFile(inputFile)
	if err != nil {
		log.Fatalf("Error reading GPX file: %v", err)
	}

	var gpx GpxRoot
	if err := xml.Unmarshal(data, &gpx); err != nil {
		log.Fatalf("Error parsing GPX: %v", err)
	}

	// Create KML document
	kml := KML{
		Xmlns: "http://www.opengis.net/kml/2.2",
		Document: Document{
			Name: strings.TrimSuffix(inputFile, ".gpx"),
		},
	}

	// Add styles
	kml.Document.Styles = []Style{
		{
			ID: "normal",
			LineStyle: LineStyle{
				Color: lineColor,
				Width: lineWidth,
			},
		},
		{
			ID: "gap",
			LineStyle: LineStyle{
				Color: gapColor,
				Width: lineWidth * 0.75, // Slightly thinner for gaps
			},
		},
	}

	// Convert tracks to placemarks
	trackNum := 0
	for _, track := range gpx.Tracks {
		for segIdx, segment := range track.Segments {
			if len(segment.Points) == 0 {
				continue
			}

			// Build coordinates string and check for gaps
			var coords []string
			var isGapSegment bool
			
			for _, point := range segment.Points {
				// Check if this is a gap connection point
				if point.Name == "Gap Connection" {
					isGapSegment = true
					
					// Add waypoint for gap connection if requested
					if showWaypoints {
						waypoint := Placemark{
							Name:        "Gap Connection",
							Description: point.Desc,
							Point: &Point{
								Coordinates: fmt.Sprintf("%f,%f,%f", 
									point.Longitude, point.Latitude, point.Elevation),
							},
						}
						kml.Document.Placemarks = append(kml.Document.Placemarks, waypoint)
					}
				}
				
				coordStr := fmt.Sprintf("%f,%f", point.Longitude, point.Latitude)
				if point.Elevation != 0 {
					coordStr = fmt.Sprintf("%f,%f,%f", point.Longitude, point.Latitude, point.Elevation)
				}
				coords = append(coords, coordStr)
			}

			// Create placemark for this segment
			name := track.Name
			if name == "" {
				trackNum++
				name = fmt.Sprintf("Track %d", trackNum)
			}
			if len(track.Segments) > 1 {
				name = fmt.Sprintf("%s - Segment %d", name, segIdx+1)
			}

			// Detect if this might be a gap connection based on segment size
			// (single segment between rides often indicates a connection)
			if len(coords) <= 2 && segIdx > 0 && segIdx < len(track.Segments)-1 {
				isGapSegment = true
			}

			placemark := Placemark{
				Name: name,
				LineString: &LineString{
					Tessellate:  1,
					Coordinates: strings.Join(coords, " "),
				},
			}

			// Set style based on whether this is a gap segment
			if isGapSegment {
				placemark.StyleURL = "#gap"
				placemark.Description = "Gap connection"
			} else {
				placemark.StyleURL = "#normal"
			}

			kml.Document.Placemarks = append(kml.Document.Placemarks, placemark)
		}
	}

	// Marshal KML to XML
	output, err := xml.MarshalIndent(kml, "", "  ")
	if err != nil {
		log.Fatalf("Error marshaling KML: %v", err)
	}

	// Add XML header
	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>` + "\n" + string(output)

	// Write KML file
	if err := os.WriteFile(outputFile, []byte(xmlContent), 0644); err != nil {
		log.Fatalf("Error writing KML file: %v", err)
	}

	fmt.Printf("Converted %s to %s\n", inputFile, outputFile)
	fmt.Printf("Found %d track(s) with %d total segment(s)\n", 
		len(gpx.Tracks), countSegments(gpx))
}

func countSegments(gpx GpxRoot) int {
	count := 0
	for _, track := range gpx.Tracks {
		count += len(track.Segments)
	}
	return count
}