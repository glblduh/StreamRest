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
	"github.com/anacrolix/torrent/metainfo"
)

var torrentCli *torrent.Client
var tcliConfs *torrent.ClientConfig

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
		modFileName := strings.Split(torrentFiles[i].DisplayPath(), "/")
		amRes.Files = append(amRes.Files, modFileName[len(modFileName)-1])
	}

	// Send response
	json.NewEncoder(w).Encode(&amRes)
}

type beginFileDownloadRes struct {
	InfoHash string
	FileName string
}

func beginFileDownload(w http.ResponseWriter, r *http.Request) {
	var eRes errorRes
	var bfdRes beginFileDownloadRes
	// Get query values
	infoHash, ihok := r.URL.Query()["infohash"]
	fileName, fnok := r.URL.Query()["filename"]

	if !ihok || !fnok {
		w.WriteHeader(404)
		w.Header().Set("Content-Type", "application/json")
		eRes.Error = "InfoHash or FileName is not provided"
		json.NewEncoder(w).Encode(&eRes)
		return
	}

	// Get torrent handler
	t, tok := torrentCli.Torrent(metainfo.NewHashFromHex(infoHash[0]))

	// Torrent not found
	w.Header().Set("Content-Type", "application/json")
	if !tok {
		w.WriteHeader(404)
		eRes.Error = "Torrent not found"
		json.NewEncoder(w).Encode(&eRes)
		return
	}

	// Get file from query
	tFiles := t.Files()
	for i := 0; i < len(tFiles); i++ {
		if strings.Contains(tFiles[i].DisplayPath(), fileName[0]) {
			tFiles[i].Download()
			break
		}
	}

	// Send response
	bfdRes.InfoHash = infoHash[0]
	bfdRes.FileName = fileName[0]
	json.NewEncoder(w).Encode(&bfdRes)
}

func beginStream(w http.ResponseWriter, r *http.Request) {
	var eRes errorRes
	// Get query values
	infoHash, ihok := r.URL.Query()["infohash"]
	fileName, fnok := r.URL.Query()["filename"]

	if !ihok || !fnok {
		w.WriteHeader(404)
		w.Header().Set("Content-Type", "application/json")
		eRes.Error = "InfoHash or FileName is not provided"
		json.NewEncoder(w).Encode(&eRes)
		return
	}

	// Get torrent handler
	t, tok := torrentCli.Torrent(metainfo.NewHashFromHex(infoHash[0]))

	// Torrent not found
	if !tok {
		w.WriteHeader(404)
		w.Header().Set("Content-Type", "application/json")
		eRes.Error = "Torrent not found"
		json.NewEncoder(w).Encode(&eRes)
		return
	}

	// Get file from query
	tFiles := t.Files()
	for i := 0; i < len(tFiles); i++ {
		if strings.Contains(tFiles[i].DisplayPath(), fileName[0]) {
			tFiles[i].Download()
			fileRead := tFiles[i].NewReader()
			fileRead.SetReadahead(tFiles[i].Length() / 100)
			fileRead.SetResponsive()
			fileRead.Seek(tFiles[i].Offset(), io.SeekStart)
			w.Header().Set("Content-Disposition", "attachment; filename=\""+t.Info().Name+"\"")
			http.ServeContent(w, r, tFiles[i].DisplayPath(), time.Now(), fileRead)
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

	// Find torrent
	t, tok := torrentCli.Torrent(metainfo.NewHashFromHex(rtBodyRes.InfoHash))

	// Torrent not found
	if !tok {
		w.WriteHeader(404)
		eRes.Error = "Torrent not found"
		json.NewEncoder(w).Encode(&eRes)
		return
	}

	// Drop from torrent client
	t.Drop()

	// Deletes files
	os.RemoveAll(filepath.Join(tcliConfs.DataDir, t.Name()))

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
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&ltRes)
}

type torrentStatsBody struct {
	InfoHash string
}

type torrentStatsRes struct {
	InfoHash      string
	Name          string
	TotalPeers    int
	ActivePeers   int
	PendingPeers  int
	HalfOpenPeers int
	Files         torrentStatsFiles
}

type torrentStatsFiles struct {
	OnTorrent []string
	OnDisk    []string
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

	// Get torrent handler
	t, tok := torrentCli.Torrent(metainfo.NewHashFromHex(tsBody.InfoHash))

	// Not found
	if !tok {
		w.WriteHeader(404)
		eRes.Error = "Torrent not found"
		json.NewEncoder(w).Encode(&eRes)
		return
	}

	// Set corresponding stats
	tsRes.InfoHash = t.InfoHash().String()
	tsRes.Name = t.Name()
	tsRes.TotalPeers = t.Stats().TotalPeers
	tsRes.ActivePeers = t.Stats().ActivePeers
	tsRes.HalfOpenPeers = t.Stats().HalfOpenPeers
	tsRes.PendingPeers = t.Stats().PendingPeers

	// Get files
	tFiles := t.Files()
	for i := 0; i < len(tFiles); i++ {
		fileName := strings.Split(tFiles[i].DisplayPath(), "/")
		tsRes.Files.OnTorrent = append(tsRes.Files.OnTorrent, fileName[len(fileName)-1])
		if tFiles[i].BytesCompleted() != 0 {
			tsRes.Files.OnDisk = append(tsRes.Files.OnDisk, fileName[len(fileName)-1])
		}
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&tsRes)
}

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
		os.MkdirAll(filepath.Join(pwd, "streamrest"), os.ModePerm)
		// Set the download directory to streamrest directory
		tcliConfs.DataDir = filepath.Join(pwd, "streamrest")
	} else {
		// Set download directory to specified directory
		tcliConfs.DataDir = filepath.Join(dataDir)
	}
	fmt.Printf("[INFO] Download directory is set to: %s\n", tcliConfs.DataDir)

	// Disable upload if specified
	if disableUpload {
		fmt.Println("[INFO] Upload is disabled")
		tcliConfs.NoUpload = true
	}

	// Make the torrent client
	torrentCli, _ = torrent.NewClient(tcliConfs)

	// HTTP Endpoints
	http.HandleFunc("/api/addmagnet", addMagnet)
	http.HandleFunc("/api/selectfile", beginFileDownload)
	http.HandleFunc("/api/stream", beginStream)
	http.HandleFunc("/api/removetorrent", removeTorrent)
	http.HandleFunc("/api/torrents", listTorrents)
	http.HandleFunc("/api/torrent", torrentStats)

	// Start listening
	fmt.Printf("[INFO] Listening on http://%s\n", httpHost)
	http.ListenAndServe(httpHost, nil)
}
