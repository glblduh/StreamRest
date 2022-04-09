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

func httpJSONError(w http.ResponseWriter, error string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if json.NewEncoder(w).Encode(errorRes{
		Error: error,
	}) != nil {
		http.Error(w, error, code)
	}
}

func addMagnet(w http.ResponseWriter, r *http.Request) {
	// Variables for JSON request body and response
	var amBody addMagnetBody
	var amRes addMagnetRes

	// Decode JSON of request body and set response Content-Type to JSON
	w.Header().Set("Content-Type", "application/json")
	if json.NewDecoder(r.Body).Decode(&amBody) != nil {
		httpJSONError(w, "Request JSON body decode error", http.StatusInternalServerError)
		return
	}

	// Response error if parameters are not given
	if amBody.Magnet == "" {
		httpJSONError(w, "Magnet link is not provided", http.StatusNotFound)
		return
	}

	// Add magnet to torrent client
	t, magErr := torrentCli.AddMagnet(amBody.Magnet)
	if magErr != nil {
		httpJSONError(w, "AddMagnet error", http.StatusInternalServerError)
		return
	}
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
					tFile.Download()
					amRes.PlaylistURL += "&file=" + url.QueryEscape(modFileName[len(modFileName)-1])
					amRes.Files = append(amRes.Files, addMagnetFiles{
						FileName:      modFileName[len(modFileName)-1],
						FileSizeBytes: int(tFile.Length()),
					})
					break
				}
			}
		}
		if json.NewEncoder(w).Encode(&amRes) != nil {
			httpJSONError(w, "Response JSON body encode error", http.StatusInternalServerError)
			return
		}
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
	if json.NewEncoder(w).Encode(&amRes) != nil {
		httpJSONError(w, "Response JSON body encode error", http.StatusInternalServerError)
		return
	}
}

func beginStream(w http.ResponseWriter, r *http.Request) {
	// Get query values
	infoHash, ihok := r.URL.Query()["infohash"]
	fileName, fnok := r.URL.Query()["file"]

	if !ihok || !fnok {
		httpJSONError(w, "InfoHash or File is not provided", http.StatusNotFound)
		return
	}

	// Get torrent handler
	if len(infoHash[0]) != 40 {
		httpJSONError(w, "InfoHash is not valid", http.StatusInternalServerError)
		return
	}
	t, tok := torrentCli.Torrent(metainfo.NewHashFromHex(infoHash[0]))

	// Torrent not found
	if !tok {
		httpJSONError(w, "Torrent not found", http.StatusNotFound)
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

	// Decode JSON of request body and set response Content-Type to JSON
	w.Header().Set("Content-Type", "application/json")
	if json.NewDecoder(r.Body).Decode(&rtBodyRes) != nil {
		httpJSONError(w, "Request JSON body decode error", http.StatusInternalServerError)
		return
	}

	// Response error if parameters are not given
	if len(rtBodyRes.InfoHash) < 1 {
		httpJSONError(w, "No InfoHash provided", http.StatusNotFound)
		return
	}

	// Get all infohashes provided
	for i, curih := range rtBodyRes.InfoHash {
		// If infohash is not 40 characters
		if len(curih) != 40 {
			rtBodyRes.InfoHash[i] = "INVALIDINFOHASH"
			continue
		}

		t, tok := torrentCli.Torrent(metainfo.NewHashFromHex(curih))

		// If torrent doesn't exist
		if !tok {
			rtBodyRes.InfoHash[i] = "NOTFOUND"
			continue
		}

		// Removal
		t.Drop()
		if os.RemoveAll(filepath.Join(tcliConfs.DataDir, t.Name())) != nil {
			rtBodyRes.InfoHash[i] = "FILEREMOVALERROR"
		}
	}

	// Send request body as response
	if json.NewEncoder(w).Encode(&rtBodyRes) != nil {
		httpJSONError(w, "Response JSON body encode error", http.StatusInternalServerError)
		return
	}
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
	if json.NewEncoder(w).Encode(&ltRes) != nil {
		httpJSONError(w, "Response JSON body encode error", http.StatusInternalServerError)
		return
	}
}

func torrentStats(w http.ResponseWriter, r *http.Request) {
	// Vars for request body and response
	infoHash, ihok := r.URL.Query()["infohash"]
	var tsRes torrentStatsRes

	w.Header().Set("Content-Type", "application/json")

	// Decodes request body to JSON
	if !ihok {
		httpJSONError(w, "InfoHash is not provided", http.StatusNotFound)
		return
	}

	// Get torrent handler
	if len(infoHash[0]) != 40 {
		httpJSONError(w, "InfoHash is not valid", http.StatusInternalServerError)
		return
	}
	t, tok := torrentCli.Torrent(metainfo.NewHashFromHex(infoHash[0]))

	// Not found
	if !tok {
		httpJSONError(w, "Torrent is not found", http.StatusNotFound)
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
	if json.NewEncoder(w).Encode(&tsRes) != nil {
		httpJSONError(w, "Response JSON body encode error", http.StatusInternalServerError)
		return
	}
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
		httpJSONError(w, "Magnet link or InfoHash is not provided", http.StatusNotFound)
		return
	}

	var t *torrent.Torrent

	// If the magnet is escaped
	if magOk && !dnOk && !trOk && !ihOk {
		var magErr error
		t, magErr = torrentCli.AddMagnet(magnet[0])
		if magErr != nil {
			httpJSONError(w, "AddMagnet error", http.StatusInternalServerError)
			return
		}
	}

	// If infohash is provided
	if ihOk && !magOk && !dnOk && !trOk {
		if len(infoHash[0]) != 40 {
			httpJSONError(w, "InfoHash is not valid", http.StatusInternalServerError)
			return
		}

		var tOk bool
		t, tOk = torrentCli.Torrent(metainfo.NewHashFromHex(infoHash[0]))

		// If infohash is not found
		if !tOk {
			httpJSONError(w, "Torrent is not found", http.StatusNotFound)
			return
		}
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
		var atsErr error
		t, _, atsErr = torrentCli.AddTorrentSpec(&torrentSpec)
		if atsErr != nil {
			httpJSONError(w, "AddTorrentSpec error", http.StatusInternalServerError)
			return
		}
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
