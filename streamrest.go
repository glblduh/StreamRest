package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/anacrolix/torrent"
	"github.com/rs/cors"
)

var torrentCli *torrent.Client
var tcliConfs *torrent.ClientConfig

func main() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		log.Println("[INFO] Termination detected. Removing torrents")
		for _, t := range torrentCli.Torrents() {
			log.Printf("[INFO] Removing torrent: [%s]\n", t.Name())
			t.Drop()
			rmaErr := os.RemoveAll(filepath.Join(tcliConfs.DataDir, t.Name()))
			if rmaErr != nil {
				log.Printf("[ERROR] Failed to remove files of torrent: [%s]: %s\n", t.Name(), rmaErr)
			}
		}
		os.Exit(0)
	}()

	dirPath := flag.String("dir", "srdir", "Set the download directory")
	httpHost := flag.String("port", ":1010", "Set the listening port")
	noUp := flag.Bool("noup", false, "Disable uploads")
	flag.Parse()

	tcliConfs = torrent.NewDefaultClientConfig()
	tcliConfs.DataDir = filepath.Clean(*dirPath)
	tcliConfs.NoUpload = *noUp

	log.Printf("[INFO] Download directory is set to: %s\n", tcliConfs.DataDir)

	_, isnoUp := os.LookupEnv("NOUP")
	if isnoUp {
		tcliConfs.NoUpload = true
	}

	if tcliConfs.NoUpload {
		log.Println("[INFO] Upload is disabled")
	}

	var tCliErr error
	torrentCli, tCliErr = torrent.NewClient(tcliConfs)
	if tCliErr != nil {
		log.Fatalf("[ERROR] Creation of BitTorrent client failed: %s\n", tCliErr)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/addmagnet", addMagnet)
	mux.HandleFunc("/api/stream", beginStream)
	mux.HandleFunc("/api/removetorrent", removeTorrent)
	mux.HandleFunc("/api/torrents", listTorrents)
	mux.HandleFunc("/api/torrent", torrentStats)
	mux.HandleFunc("/api/play", playMagnet)

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "DELETE"},
		AllowCredentials: true,
	})

	log.Printf("[INFO] Listening on http://%s\n", *httpHost)
	log.Fatalln(http.ListenAndServe(*httpHost, c.Handler(mux)))
}
