package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
)

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

	// If selected files
	if len(amBody.Files) > 0 {
		amRes.PlaylistURL = "/api/play?infohash=" + t.InfoHash().String()
		for _, selFile := range amBody.Files {
			for _, tFile := range t.Files() {
				modFileName := strings.Split(tFile.DisplayPath(), "/")
				if strings.Contains(strings.ToLower(modFileName[len(modFileName)-1]), strings.ToLower(selFile)) {
					amRes.PlaylistURL += "&file=" + url.QueryEscape(modFileName[len(modFileName)-1])
					amRes.Files = append(amRes.Files, addMagnetFiles{
						FileName:      modFileName[len(modFileName)-1],
						FileSizeBytes: int(tFile.Length()),
					})
					break
				}
			}
		}
		json.NewEncoder(w).Encode(&amRes)
		return
	}

	// If all files are selected
	if amBody.AllFiles {
		t.DownloadAll()
		amRes.PlaylistURL = "/api/play?infohash=" + t.InfoHash().String()
	}

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
		modFileName := strings.Split(tFile.DisplayPath(), "/")
		if strings.Compare(modFileName[len(modFileName)-1], fileName[0]) == 0 {
			w.Header().Set("Content-Disposition", "attachment; filename=\""+modFileName[len(modFileName)-1]+"\"")
			fileRead := tFile.NewReader()
			defer fileRead.Close()
			fileRead.SetReadahead(tFile.Length() / 100)
			fileRead.SetResponsive()
			http.ServeContent(w, r, modFileName[len(modFileName)-1], time.Now(), fileRead)
			break
		}
	}
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

func playMagnet(w http.ResponseWriter, r *http.Request) {
	// Needed variables
	infoHash, ihOk := r.URL.Query()["infohash"]
	magnet, magOk := r.URL.Query()["magnet"]
	displayName, dnOk := r.URL.Query()["dn"]
	trackers, trOk := r.URL.Query()["tr"]
	files, fOk := r.URL.Query()["file"]

	// Check if provided with magnet
	if !magOk && !ihOk {
		eRes := errorRes{
			Error: "Magnet link or InfoHash is not provided",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(&eRes)
		return
	}

	var t *torrent.Torrent

	// If the magnet is escaped
	if magOk && !dnOk && !trOk && !ihOk {
		t, _ = torrentCli.AddMagnet(magnet[0])
	}

	// If infohash is provided
	if ihOk && !magOk && !dnOk && !trOk {
		t, _ = torrentCli.Torrent(metainfo.NewHashFromHex(infoHash[0]))
	}

	// If the magnet is not escaped
	if magOk && !ihOk && dnOk || trOk {
		torrentSpec := torrent.TorrentSpec{}

		// Get infohash from magnet
		magnetSplit := strings.Split(magnet[0], ":")
		torrentSpec.InfoHash = metainfo.NewHashFromHex(magnetSplit[len(magnetSplit)-1])

		// Set display name if present
		if dnOk {
			torrentSpec.DisplayName = displayName[0]
		}

		// Set custom trackers if present
		if trOk {
			for i, tracker := range trackers {
				torrentSpec.Trackers = append(torrentSpec.Trackers, []string{})
				torrentSpec.Trackers[i] = append(torrentSpec.Trackers[i], tracker)
			}
		}

		// Add torrent spec
		t, _, _ = torrentCli.AddTorrentSpec(&torrentSpec)
	}

	<-t.GotInfo()

	// Create playlist file
	w.Header().Set("Content-Disposition", "attachment; filename=\""+t.InfoHash().String()+".m3u\"")
	playList := "#EXTM3U\n"

	// Check HTTP scheme if behind reverse proxy
	httpScheme := "http"
	if r.Header.Get("X-Forwarded-Proto") != "" {
		httpScheme = r.Header.Get("X-Forwarded-Proto")
	}

	// If only one file
	if !t.Info().IsDir() {
		tFile := t.Files()[0]
		tFile.Download()
		modFileName := strings.Split(tFile.DisplayPath(), "/")
		playList += "#EXTINF:-1," + modFileName[len(modFileName)-1] + "\n"
		playList += httpScheme + "://" + r.Host + "/api/stream?infohash=" + t.InfoHash().String() + "&file=" + url.QueryEscape(modFileName[len(modFileName)-1]) + "\n"
		w.Write([]byte(playList))
		return
	}

	// If no files are selected
	if t.Info().IsDir() && !fOk {
		t.DownloadAll()
		for _, tFile := range t.Files() {
			modFileName := strings.Split(tFile.DisplayPath(), "/")
			playList += "#EXTINF:-1," + modFileName[len(modFileName)-1] + "\n"
			playList += httpScheme + "://" + r.Host + "/api/stream?infohash=" + t.InfoHash().String() + "&file=" + url.QueryEscape(modFileName[len(modFileName)-1]) + "\n"
		}
		w.Write([]byte(playList))
		return
	}

	for _, file := range files {
		for _, tFile := range t.Files() {
			modFileName := strings.Split(tFile.DisplayPath(), "/")
			if strings.Contains(strings.ToLower(modFileName[len(modFileName)-1]), strings.ToLower(file)) {
				playList += "#EXTINF:-1," + modFileName[len(modFileName)-1] + "\n"
				playList += httpScheme + "://" + r.Host + "/api/stream?infohash=" + t.InfoHash().String() + "&file=" + url.QueryEscape(modFileName[len(modFileName)-1]) + "\n"
				tFile.Download()
				break
			}
		}
	}
	w.Write([]byte(playList))
}
