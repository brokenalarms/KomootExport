# Komoot Export

Create a new `creds.yaml` file, insert your credentials and run the downloader.

## Usage

You can run the downloader with custom parameters:

```
go run . --toursDir <download-directory> --creds <path-to-creds.yaml> --tourType <tour_recorded|tour_planned> --includeTitleInDir
```

- `--toursDir`: Directory where tours will be downloaded (default: `tours`)
- `--creds`: Path to your credentials file (default: `creds.yaml`)
- `--tourType`: Type of tours to download (`tour_recorded` or `tour_planned`, default: `tour_recorded`)
- `--includeTitleInDir`: If set, the tour title will be included in the download directory name (default: not set)

## Features

This script downloads the following for each tour:

- GPX file
- Map image
- All cover images

A short wait time is added between downloads to avoid server rate limits.

## How to get your Komoot user_id and cookie

To use this script, you need your Komoot user ID and session cookie. Here's how to get them:

### user_id

1. Log in to <https://www.komoot.com> in your browser.
2. Go to <https://www.komoot.com/account/details>.
3. Look for your user ID on the page. It is a long number (e.g., `123456789`).

### cookie

1. Log in to <https://www.komoot.com> in your browser.
2. Open the browser developer tools (usually F12 or Ctrl+Shift+I).
3. Go to the "Network" tab.
4. Reload the page or perform any action that sends a request.
5. Click on any request to <https://www.komoot.com>.
6. In the request details, find the "Request Headers" section.
7. Look for the "Cookie" header and copy its entire value.
8. Paste the value into your `creds.yaml` file. It should start with or contain `kmt_sess=...`.

### Example credentials file

Your `creds.yaml` should look like this:

```yaml
user_id: "your_user_id"
cookie: "your_cookie_string"
```

# Merging multiple gpx files by date range

Once the gpx files are downloaded, you can use the script `merge_by_date.go` to merge multiple gpx files together by date range, e.g., for purposes of making an animation of a multi-day tour.

Usage is `go run merge_by_date.go [--connect] <start_date> <end_date> <output.gpx>`
`--connect` will connect gaps with a straight line between rides with marked segments (like if you took the train between points on a tour but don't want a gap displayed).

# Converting gpx to kml
Run `go run ./gpx2kml.go` to put gpx files into KML format. For a list of args here, run `go run ./gpx2kml.go -h`.

## Troubleshooting

Sometimes Komoot servers return random errors. In that case, delete the folder of the last downloaded tour and run the script again.
