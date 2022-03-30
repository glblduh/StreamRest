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
	Files  []string
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

type addMagnetOneFileRes struct {
	InfoHash  string
	Name      string
	StreamURL string
}

type addMagnetSelFiles struct {
	InfoHash    string
	Name        string
	StreamURL   []string
	PlaylistURL string
}

func addMagnet(w http.ResponseWriter, r *http.Request) {
	// Variables for JSON request body and response
	var amBody addMagnetBody
	var amRes addMagnetRes
	var eRes errorRes
	var amOFRes addMagnetOneFileRes
	var amSFRes addMagnetSelFiles

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

	// If torrent have only one file, start downloading the file
	if !t.Info().IsDir() {
		t.Files()[0].Download()
		amOFRes.InfoHash = t.InfoHash().String()
		amOFRes.Name = t.Name()
		modFileName := strings.Split(t.Files()[0].DisplayPath(), "/")
		amOFRes.StreamURL = "/api/stream?infohash=" + t.InfoHash().String() + "&file=" + url.QueryEscape(modFileName[len(modFileName)-1])
		json.NewEncoder(w).Encode(&amOFRes)
		return
	}

	// If request have pre-selected files for stream
	if len(amBody.Files) > 0 {
		amSFRes.InfoHash = t.InfoHash().String()
		amSFRes.Name = t.Name()
		amSFRes.PlaylistURL = "/api/playlist?infohash=" + t.InfoHash().String()

		// Get file from query
		for i, ambFile := range amBody.Files {
			amSFRes.StreamURL = append(amSFRes.StreamURL, "NOT FOUND")
			for _, tFile := range t.Files() {
				if ambFile != "" && strings.Contains(strings.ToLower(tFile.DisplayPath()), strings.ToLower(ambFile)) {
					amSFRes.StreamURL[i] = "/api/stream?infohash=" + t.InfoHash().String() + "&file=" + url.QueryEscape(ambFile)
					amSFRes.PlaylistURL += "&file=" + url.QueryEscape(ambFile)
					tFile.Download()
					break
				}
			}
		}
		json.NewEncoder(w).Encode(&amSFRes)
		return
	}

	// Make response
	amRes.InfoHash = t.InfoHash().String()
	amRes.Name = t.Name()

	// Get all files
	for _, tFile := range t.Files() {
		modFileName := strings.Split(tFile.DisplayPath(), "/")
		amRes.Files = append(amRes.Files, addMagnetFiles{
			FileName:      modFileName[len(modFileName)-1],
			FileSizeBytes: int(tFile.Length()),
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
	InfoHash    string
	AllFiles    bool
	Files       []string
	StreamURL   []string
	PlaylistURL string
}

func beginFileDownload(w http.ResponseWriter, r *http.Request) {
	var eRes errorRes
	var bfdBody beginFileDownloadBody
	var bfdRes beginFileDownloadRes

	w.Header().Set("Content-Type", "application/json")
	// Parse JSON body
	json.NewDecoder(r.Body).Decode(&bfdBody)
	if bfdBody.InfoHash == "" || !bfdBody.AllFiles && len(bfdBody.Files) < 1 {
		w.WriteHeader(404)
		eRes.Error = "InfoHash or Files is not provided"
		json.NewEncoder(w).Encode(&eRes)
		return
	}

	// Get torrent handler
	t, tok := torrentCli.Torrent(metainfo.NewHashFromHex(bfdBody.InfoHash))

	// Torrent not found
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
		bfdRes.PlaylistURL = "/api/playlist?infohash=" + bfdBody.InfoHash
		for _, tFile := range t.Files() {
			modFileName := strings.Split(tFile.DisplayPath(), "/")
			bfdRes.PlaylistURL += "&file=" + url.QueryEscape(modFileName[len(modFileName)-1])
		}
		json.NewEncoder(w).Encode(&bfdRes)
		return
	}

	// Get file from query
	bfdRes.PlaylistURL = "/api/playlist?infohash=" + bfdBody.InfoHash
	for i, bfdbFile := range bfdBody.Files {
		bfdRes.Files = append(bfdRes.Files, "NOT FOUND")
		bfdRes.StreamURL = append(bfdRes.StreamURL, "NOT FOUND")
		for _, tFile := range t.Files() {
			if bfdbFile != "" && strings.Contains(strings.ToLower(tFile.DisplayPath()), strings.ToLower(bfdbFile)) {
				bfdRes.Files[i] = bfdbFile
				bfdRes.StreamURL[i] = "/api/stream?infohash=" + bfdBody.InfoHash + "&file=" + url.QueryEscape(bfdbFile)
				bfdRes.PlaylistURL += "&file=" + url.QueryEscape(bfdbFile)
				tFile.Download()
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
	fileName, fnok := r.URL.Query()["file"]

	if !ihok || !fnok {
		w.WriteHeader(404)
		w.Header().Set("Content-Type", "application/json")
		eRes.Error = "InfoHash or File is not provided"
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
	for _, tFile := range t.Files() {
		if strings.Contains(strings.ToLower(tFile.DisplayPath()), strings.ToLower(fileName[0])) {
			fileRead := tFile.NewReader()
			defer fileRead.Close()
			fileRead.SetReadahead(tFile.Length() / 100)
			fileRead.SetResponsive()
			fileRead.Seek(tFile.Offset(), io.SeekStart)
			w.Header().Set("Content-Disposition", "attachment; filename=\""+t.Info().Name+"\"")
			http.ServeContent(w, r, tFile.DisplayPath(), time.Now(), fileRead)
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
	for _, t := range torrentCli.Torrents() {
		ltRes.Torrents = append(ltRes.Torrents, listTorrentNameInfoHash{
			Name:     t.Name(),
			InfoHash: t.InfoHash().String(),
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
	StreamURL       string
	BytesDownloaded int
	FileSizeBytes   int
}

func torrentStats(w http.ResponseWriter, r *http.Request) {
	// Vars for request body and response
	infoHash, ihok := r.URL.Query()["infohash"]
	var tsRes torrentStatsRes
	var eRes errorRes

	w.Header().Set("Content-Type", "application/json")

	// Decodes request body to JSON
	if !ihok {
		w.WriteHeader(404)
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
	for _, tFile := range t.Files() {
		fileName := strings.Split(tFile.DisplayPath(), "/")
		tsRes.Files.OnTorrent = append(tsRes.Files.OnTorrent, torrentStatsFilesOnTorrent{
			FileName:      fileName[len(fileName)-1],
			FileSizeBytes: int(tFile.Length()),
		})
		if tFile.BytesCompleted() != 0 {
			tsRes.Files.OnDisk = append(tsRes.Files.OnDisk, torrentStatsFilesOnDisk{
				FileName:        fileName[len(fileName)-1],
				StreamURL:       "/api/stream?infohash=" + t.InfoHash().String() + "&file=" + url.QueryEscape(fileName[len(fileName)-1]),
				BytesDownloaded: int(tFile.BytesCompleted()),
				FileSizeBytes:   int(tFile.Length()),
			})
		}
	}

	// Send response
	json.NewEncoder(w).Encode(&tsRes)
}

func getFilePlaylist(w http.ResponseWriter, r *http.Request) {
	var eRes errorRes
	// Get query values
	infoHash, ihok := r.URL.Query()["infohash"]
	files, fok := r.URL.Query()["file"]

	// Check presence
	if !ihok || !fok {
		w.WriteHeader(404)
		eRes.Error = "InfoHash or Files not provided"
		w.Header().Set("Content-Type", "application/json")
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

	// Check HTTP scheme if behind reverse proxy
	httpScheme := "http"
	if r.Header.Get("X-Forwarded-Proto") != "" {
		httpScheme = r.Header.Get("X-Forwarded-Proto")
	}

	// Create M3U file
	w.Header().Set("Content-Disposition", "attachment; filename=\""+infoHash[0]+".m3u\"")
	playList := "#EXTM3U\n"
	for _, file := range files {
		for _, tFile := range t.Files() {
			modFileName := strings.Split(tFile.DisplayPath(), "/")
			if strings.Contains(strings.ToLower(modFileName[len(modFileName)-1]), strings.ToLower(file)) {
				playList += "#EXTINF:-1," + modFileName[len(modFileName)-1] + "\n"
				playList += httpScheme + "://" + r.Host + "/api/stream?infohash=" + infoHash[0] + "&file=" + url.QueryEscape(modFileName[len(modFileName)-1]) + "\n"
				tFile.Download()
				break
			}
		}
	}
	fmt.Fprint(w, playList)
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
	mux.HandleFunc("/api/playlist", getFilePlaylist)

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
