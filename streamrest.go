package main

import (
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

	httpHost := ":1010"
	dataDir := ""
	disableUpload := false

	_, isnoUp := os.LookupEnv("NOUP")
	if isnoUp {
		disableUpload = true
	}

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

	tcliConfs = torrent.NewDefaultClientConfig()

	if dataDir == "" {
		pwd, pwdErr := os.Getwd()
		if pwdErr != nil {
			log.Fatalf("[ERROR] Cannot get working directory: %s\n", pwdErr)
		}
		mkaErr := os.MkdirAll(filepath.Join(pwd, "srdir"), os.ModePerm)
		if mkaErr != nil {
			log.Fatalf("[ERROR] Creation of download directory failed: %s\n", mkaErr)
		}
		tcliConfs.DataDir = filepath.Join(pwd, "srdir")
	} else {
		tcliConfs.DataDir = filepath.Join(dataDir)
	}
	log.Printf("[INFO] Download directory is set to: %s\n", tcliConfs.DataDir)

	if disableUpload {
		log.Println("[INFO] Upload is disabled")
		tcliConfs.NoUpload = true
	}

	var tCliErr error
	torrentCli, tCliErr = torrent.NewClient(tcliConfs)
	if tCliErr != nil {
		log.Fatalf("[ERROR] Creation of TorrentClient failed: %s\n", tCliErr)
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

	log.Printf("[INFO] Listening on http://%s\n", httpHost)
	log.Fatalln(http.ListenAndServe(httpHost, c.Handler(mux)))
}
