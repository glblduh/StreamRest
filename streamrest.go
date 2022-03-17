package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anacrolix/torrent"
)

var torrentCli *torrent.Client

type errorRes struct {
	Error string
}

type addMagnetBody struct {
	Magnet string
}

type addMagnetRes struct {
	InfoHash string
	Files    []string
}

func addMagnet(w http.ResponseWriter, r *http.Request) {
	// Variables for JSON request body and response
	var amBody addMagnetBody
	var amRes addMagnetRes
	var eRes errorRes

	// Decode JSON of request body and set response Content-Type to JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewDecoder(r.Body).Decode(&amBody)

	// Response error if parameters are not given
	if amBody.Magnet == "" {
		eRes.Error = "Magnet is not provided"
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(&eRes)
		return
	}

	// Add magnet to torrent client
	t, _ := torrentCli.AddMagnet(amBody.Magnet)
	<-t.GotInfo()

	// Make response
	amRes.InfoHash = t.InfoHash().String()

	// Get all files
	torrentFiles := t.Files()
	for i := 0; i < len(torrentFiles); i++ {
		if strings.Contains(torrentFiles[i].DisplayPath(), "/") {
			modFileName := strings.Split(torrentFiles[i].DisplayPath(), "/")
			amRes.Files = append(amRes.Files, modFileName[len(modFileName)-1])
		} else {
			amRes.Files = append(amRes.Files, torrentFiles[i].DisplayPath())
		}
	}

	// Send response
	json.NewEncoder(w).Encode(&amRes)
}

func beginFileDownload(w http.ResponseWriter, r *http.Request) {
	var eRes errorRes
	// Get query values
	infoHash, ihok := r.URL.Query()["infohash"]
	fileName, fnok := r.URL.Query()["filename"]

	if !ihok || !fnok {
		w.WriteHeader(404)
		eRes.Error = "InfoHash or FileName is not provided"
		json.NewEncoder(w).Encode(&eRes)
		return
	}

	// Find file from torrents then start downloads
	var allTorrents = torrentCli.Torrents()
	var torrentFiles []*torrent.File
	for i := 0; i < len(allTorrents); i++ {
		if allTorrents[i].InfoHash().String() == infoHash[0] {
			torrentFiles = allTorrents[i].Files()
			for j := 0; j < len(torrentFiles); j++ {
				if strings.Contains(torrentFiles[j].DisplayPath(), fileName[0]) {
					torrentFiles[j].Download()
					fileRead := torrentFiles[j].NewReader()
					fileRead.SetReadahead(torrentFiles[j].Length() / 100)
					fileRead.SetResponsive()
					fileRead.Seek(torrentFiles[j].Offset(), io.SeekStart)
					http.ServeContent(w, r, torrentFiles[j].DisplayPath(), time.Now(), fileRead)
					break
				}
			}
			break
		}
	}
}

type removeTorrentBodyRes struct {
	InfoHash string
}

func removeTorrent(w http.ResponseWriter, r *http.Request) {
	// Vars for request and response
	var rtBodyRes removeTorrentBodyRes
	var eRes errorRes

	// Decode JSON of request body and set response Content-Type to JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewDecoder(r.Body).Decode(&rtBodyRes)

	// Response error if parameters are not given
	if rtBodyRes.InfoHash == "" {
		eRes.Error = "InfoHash is not provided"
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(&eRes)
		return
	}

	// Find torrent and remove it
	allTorrents := torrentCli.Torrents()
	for i := 0; i < len(allTorrents); i++ {
		if allTorrents[i].InfoHash().String() == rtBodyRes.InfoHash {
			allTorrents[i].Drop()
			os.Remove(filepath.Join(".", "streamrest", "downloads", allTorrents[i].Name()))
			os.RemoveAll(filepath.Join(".", "streamrest", "downloads", allTorrents[i].Name()))
			break
		}
	}

	// Send request body as response
	json.NewEncoder(w).Encode(&rtBodyRes)
}

type listTorrentsRes struct {
	Torrents []string
}

func listTorrents(w http.ResponseWriter, r *http.Request) {
	// Var for JSON response
	var ltRes listTorrentsRes

	// Get infohash of all of the torrents
	allTorrents := torrentCli.Torrents()
	for i := 0; i < len(allTorrents); i++ {
		ltRes.Torrents = append(ltRes.Torrents, allTorrents[i].InfoHash().String())
	}

	//Send response
	json.NewEncoder(w).Encode(&ltRes)
}

type torrentStatsBody struct {
	InfoHash string
}

type torrentStatsRes struct {
	InfoHash      string
	Name          string
	isSeeding     bool
	TotalPeers    int
	ActivePeers   int
	PendingPeers  int
	HalfOpenPeers int
}

func torrentStats(w http.ResponseWriter, r *http.Request) {
	// Vars for request body and response
	var tsBody torrentStatsBody
	var tsRes torrentStatsRes
	var eRes errorRes

	// Decodes request body to JSON
	json.NewDecoder(r.Body).Decode(&tsBody)
	if tsBody.InfoHash == "" {
		eRes.Error = "InfoHash is not provided"
		json.NewEncoder(w).Encode(&eRes)
		return
	}

	// Get info from torrent selected
	allTorrents := torrentCli.Torrents()
	for i := 0; i < len(allTorrents); i++ {
		if allTorrents[i].InfoHash().String() == tsBody.InfoHash {
			tsRes.InfoHash = allTorrents[i].InfoHash().String()
			tsRes.Name = allTorrents[i].Name()
			tsRes.isSeeding = allTorrents[i].Seeding()
			tsRes.TotalPeers = allTorrents[i].Stats().TotalPeers
			tsRes.ActivePeers = allTorrents[i].Stats().ActivePeers
			tsRes.HalfOpenPeers = allTorrents[i].Stats().HalfOpenPeers
			tsRes.PendingPeers = allTorrents[i].Stats().PendingPeers
			break
		}
	}

	// Send response
	json.NewEncoder(w).Encode(&tsRes)
}

func main() {
	// Make streamrest directory if doesn't exist
	os.MkdirAll(filepath.Join(".", "streamrest"), os.ModePerm)
	os.MkdirAll(filepath.Join(".", "streamrest", "downloads"), os.ModePerm)

	// Make config
	tcliConfs := torrent.NewDefaultClientConfig()

	// Set the download directory to streamrest directory
	tcliConfs.DataDir = filepath.Join(".", "streamrest", "downloads")

	// Check for NOUPLOAD env key to disable upload
	_, existEnvNoUpload := os.LookupEnv("NOUPLOAD")
	if existEnvNoUpload {
		fmt.Println("[INFO] Upload is disabled")
		tcliConfs.NoUpload = existEnvNoUpload
	}

	// Make the torrent client
	torrentCli, _ = torrent.NewClient(tcliConfs)

	// HTTP Endpoints
	http.HandleFunc("/api/addmagnet", addMagnet)
	http.HandleFunc("/api/stream", beginFileDownload)
	http.HandleFunc("/api/removetorrent", removeTorrent)
	http.HandleFunc("/api/torrents", listTorrents)
	http.HandleFunc("/api/torrent", torrentStats)

	// Start listening
	httpHost := ":1010"
	envHost, existEnvHost := os.LookupEnv("HOST")
	if existEnvHost {
		httpHost = envHost
	}
	fmt.Printf("[INFO] Listening on http://%s\n", httpHost)
	http.ListenAndServe(httpHost, nil)
}
