package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/anacrolix/torrent"
	"github.com/rs/cors"
)

var torrentCli *torrent.Client
var tcliConfs *torrent.ClientConfig

func main() {
	// Vars
	httpHost := ":1010"
	dataDir := ""
	disableUpload := false

	// NOUP from env
	_, isnoUp := os.LookupEnv("NOUP")
	if isnoUp {
		disableUpload = true
	}

	// Parse arguments
	progArgs := os.Args[1:]
	for i := 0; i < len(progArgs); i++ {
		if progArgs[i] == "-l" {
			httpHost = progArgs[i+1]
		}
		if progArgs[i] == "-d" {
			dataDir = progArgs[i+1]
		}
		if progArgs[i] == "--noup" {
			disableUpload = true
		}
	}

	// Make config
	tcliConfs = torrent.NewDefaultClientConfig()

	if dataDir == "" {
		// Get current working directory
		pwd, _ := os.Getwd()
		// Make streamrest directory if doesn't exist
		if os.MkdirAll(filepath.Join(pwd, "srdir"), os.ModePerm) != nil {
			log.Fatalln("[ERROR] Creation of download directory failed")
		}
		// Set the download directory to streamrest directory
		tcliConfs.DataDir = filepath.Join(pwd, "srdir")
	} else {
		// Set download directory to specified directory
		tcliConfs.DataDir = filepath.Join(dataDir)
	}
	log.Printf("[INFO] Download directory is set to: %s\n", tcliConfs.DataDir)

	// Disable upload if specified
	if disableUpload {
		log.Println("[INFO] Upload is disabled")
		tcliConfs.NoUpload = true
	}

	// Make the torrent client
	var tCliErr error
	torrentCli, tCliErr = torrent.NewClient(tcliConfs)
	if tCliErr != nil {
		log.Fatalln("[ERROR] Creation of TorrentClient failed")
	}

	// HTTP Endpoints
	mux := http.NewServeMux()
	mux.HandleFunc("/api/addmagnet", addMagnet)
	mux.HandleFunc("/api/stream", beginStream)
	mux.HandleFunc("/api/removetorrent", removeTorrent)
	mux.HandleFunc("/api/torrents", listTorrents)
	mux.HandleFunc("/api/torrent", torrentStats)
	mux.HandleFunc("/api/play", playMagnet)

	// CORS
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "DELETE"},
		AllowCredentials: true,
	})

	// Start listening
	log.Printf("[INFO] Listening on http://%s\n", httpHost)
	log.Fatalln(http.ListenAndServe(httpHost, c.Handler(mux)))
}
