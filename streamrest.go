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

func main() {
	// Make streamrest directory if doesn't exist
	os.MkdirAll(filepath.Join(".", "streamrest"), os.ModePerm)
	os.MkdirAll(filepath.Join(".", "streamrest", "downloads"), os.ModePerm)

	// Make config
	tcliConfs := torrent.NewDefaultClientConfig()

	// Set the download directory to streamrest directory
	fmt.Println(filepath.Join(".", "streamrest", "downloads"))
	tcliConfs.DataDir = filepath.Join(".", "streamrest", "downloads")

	// Make the torrent client
	torrentCli, _ = torrent.NewClient(tcliConfs)

	// HTTP Endpoints
	http.HandleFunc("/api/addmagnet", addMagnet)
	http.HandleFunc("/api/stream", beginFileDownload)
	http.HandleFunc("/api/removetorrent", removeTorrent)

	// Listen to port 1010
	fmt.Printf("StreamRest is listening at http://0.0.0.0:1010\n")
	http.ListenAndServe(":1010", nil)
}
