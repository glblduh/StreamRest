package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/rs/cors"
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
	Name     string
	Files    []addMagnetFiles
}

type addMagnetFiles struct {
	FileName      string
	FileSizeBytes int
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
	amRes.Name = t.Name()

	// Get all files
	torrentFiles := t.Files()
	for i := 0; i < len(torrentFiles); i++ {
		modFileName := strings.Split(torrentFiles[i].DisplayPath(), "/")
		amRes.Files = append(amRes.Files, addMagnetFiles{
			FileName:      modFileName[len(modFileName)-1],
			FileSizeBytes: int(torrentFiles[i].Length()),
		})
	}

	// Send response
	json.NewEncoder(w).Encode(&amRes)
}

type beginFileDownloadBody struct {
	InfoHash string
	Files    []string
	AllFiles bool
}

type beginFileDownloadRes struct {
	InfoHash  string
	AllFiles  bool
	Files     []string
	StreamURL []string
}

func beginFileDownload(w http.ResponseWriter, r *http.Request) {
	var eRes errorRes
	var bfdBody beginFileDownloadBody
	var bfdRes beginFileDownloadRes

	// Parse JSON body
	json.NewDecoder(r.Body).Decode(&bfdBody)
	if bfdBody.InfoHash == "" || len(bfdBody.Files) < 1 {
		w.WriteHeader(404)
		w.Header().Set("Content-Type", "application/json")
		eRes.Error = "InfoHash or Files is not provided"
		json.NewEncoder(w).Encode(&eRes)
		return
	}

	// Get torrent handler
	t, tok := torrentCli.Torrent(metainfo.NewHashFromHex(bfdBody.InfoHash))

	// Torrent not found
	w.Header().Set("Content-Type", "application/json")
	if !tok {
		w.WriteHeader(404)
		eRes.Error = "Torrent not found"
		json.NewEncoder(w).Encode(&eRes)
		return
	}

	// If all files downloaded
	if bfdBody.AllFiles {
		t.DownloadAll()
		bfdRes.InfoHash = bfdBody.InfoHash
		bfdRes.AllFiles = bfdBody.AllFiles
		json.NewEncoder(w).Encode(&bfdRes)
		return
	}

	// Get file from query
	tFiles := t.Files()
	for i := 0; i < len(bfdBody.Files); i++ {
		bfdRes.Files = append(bfdRes.Files, "NOT FOUND")
		bfdRes.StreamURL = append(bfdRes.StreamURL, "NOT FOUND")
		for j := 0; j < len(tFiles); j++ {
			if bfdBody.Files[i] != "" && strings.Contains(strings.ToLower(tFiles[j].DisplayPath()), strings.ToLower(bfdBody.Files[i])) {
				bfdRes.Files[i] = bfdBody.Files[i]
				bfdRes.StreamURL[i] = "/api/stream?infohash=" + bfdBody.InfoHash + "&filename=" + url.QueryEscape(bfdBody.Files[i])
				tFiles[j].Download()
				break
			}
		}
	}

	// Send response
	bfdRes.InfoHash = bfdBody.InfoHash
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
		if strings.Contains(strings.ToLower(tFiles[i].DisplayPath()), strings.ToLower(fileName[0])) {
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
	Torrents []listTorrentNameInfoHash
}

type listTorrentNameInfoHash struct {
	Name     string
	InfoHash string
}

func listTorrents(w http.ResponseWriter, r *http.Request) {
	// Var for JSON response
	var ltRes listTorrentsRes

	// Get infohash of all of the torrents
	allTorrents := torrentCli.Torrents()
	for i := 0; i < len(allTorrents); i++ {
		ltRes.Torrents = append(ltRes.Torrents, listTorrentNameInfoHash{
			Name:     allTorrents[i].Name(),
			InfoHash: allTorrents[i].InfoHash().String(),
		})
	}

	//Send response
	if len(ltRes.Torrents) < 1 {
		w.WriteHeader(404)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&ltRes)
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
	OnTorrent []torrentStatsFilesOnTorrent
	OnDisk    []torrentStatsFilesOnDisk
}

type torrentStatsFilesOnTorrent struct {
	FileName      string
	FileSizeBytes int
}

type torrentStatsFilesOnDisk struct {
	FileName        string
	BytesDownloaded int
	FileSizeBytes   int
}

func torrentStats(w http.ResponseWriter, r *http.Request) {
	// Vars for request body and response
	infoHash, ihok := r.URL.Query()["infohash"]
	var tsRes torrentStatsRes
	var eRes errorRes

	// Decodes request body to JSON
	if !ihok {
		eRes.Error = "InfoHash is not provided"
		json.NewEncoder(w).Encode(&eRes)
		return
	}

	// Get torrent handler
	t, tok := torrentCli.Torrent(metainfo.NewHashFromHex(infoHash[0]))

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
		tsRes.Files.OnTorrent = append(tsRes.Files.OnTorrent, torrentStatsFilesOnTorrent{
			FileName:      fileName[len(fileName)-1],
			FileSizeBytes: int(tFiles[i].Length()),
		})
		if tFiles[i].BytesCompleted() != 0 {
			tsRes.Files.OnDisk = append(tsRes.Files.OnDisk, torrentStatsFilesOnDisk{
				FileName:        fileName[len(fileName)-1],
				BytesDownloaded: int(tFiles[i].BytesCompleted()),
				FileSizeBytes:   int(tFiles[i].Length()),
			})
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
	mux := http.NewServeMux()
	mux.HandleFunc("/api/addmagnet", addMagnet)
	mux.HandleFunc("/api/selectfile", beginFileDownload)
	mux.HandleFunc("/api/stream", beginStream)
	mux.HandleFunc("/api/removetorrent", removeTorrent)
	mux.HandleFunc("/api/torrents", listTorrents)
	mux.HandleFunc("/api/torrent", torrentStats)

	// CORS
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "DELETE"},
		AllowCredentials: true,
	})

	// Start listening
	fmt.Printf("[INFO] Listening on http://%s\n", httpHost)
	http.ListenAndServe(httpHost, c.Handler(mux))
}
