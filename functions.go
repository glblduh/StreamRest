package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
)

func safenDisplayPath(displayPath string) string {
	fileNameArray := strings.Split(displayPath, "/")
	return strings.Join(fileNameArray, " ")
}

func appendFilePlaylist(scheme string, host string, infohash string, name string) string {
	playList := "#EXTINF:-1," + safenDisplayPath(name) + "\n"
	playList += scheme + "://" + host + "/api/stream?infohash=" + infohash + "&file=" + url.QueryEscape(name) + "\n"
	return playList
}

func nameCheck(str string, substr string) bool {
	splittedSubStr := strings.Split(substr, " ")
	for _, curWord := range splittedSubStr {
		if !strings.Contains(str, curWord) {
			return false
		}
	}
	return true
}

func getTorrentFile(files []*torrent.File, filename string, exactName bool) *torrent.File {
	var tFile *torrent.File = nil
	for _, file := range files {
		if exactName && file.DisplayPath() == filename {
			tFile = file
		}
		if !exactName && filename != "" && nameCheck(strings.ToLower(file.DisplayPath()), strings.ToLower(filename)) {
			tFile = file
		}
		if tFile != nil {
			break
		}
	}
	return tFile
}

func makePlayStreamURL(infohash string, filename string, isStream bool) string {
	endPoint := "play"
	if isStream {
		endPoint = "stream"
	}
	URL := "/api/" + endPoint + "?infohash=" + infohash
	if filename != "" {
		URL += "&file=" + url.QueryEscape(filename)
	}
	return URL
}

func httpJSONError(w http.ResponseWriter, error string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if json.NewEncoder(w).Encode(errorRes{
		Error: error,
	}) != nil {
		http.Error(w, error, code)
	}
}

func parseRequestBody(w http.ResponseWriter, r *http.Request, v any) error {
	err := json.NewDecoder(r.Body).Decode(v)
	if err != nil {
		httpJSONError(w, "Request JSON body decode error", http.StatusInternalServerError)
	}
	return err
}

func makeJSONResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(v)
	if err != nil {
		httpJSONError(w, "Response JSON body encode error", http.StatusInternalServerError)
	}
}

func getInfo(t *torrent.Torrent) {
	if t != nil {
		<-t.GotInfo()
	}
}

func initMagnet(w http.ResponseWriter, magnet string, alldn []string, alltr []string) *torrent.Torrent {
	var t *torrent.Torrent = nil
	var err error
	magnetString := magnet
	for _, dn := range alldn {
		magnetString += "&dn=" + url.QueryEscape(dn)
	}
	for _, tr := range alltr {
		magnetString += "&tr=" + url.QueryEscape(tr)
	}
	t, err = torrentCli.AddMagnet(magnetString)
	if err != nil {
		httpJSONError(w, "Torrent add error", http.StatusInternalServerError)
	}
	getInfo(t)
	return t
}

func getTorrent(w http.ResponseWriter, infoHash string) *torrent.Torrent {
	var t *torrent.Torrent = nil
	var tOk bool
	if len(infoHash) != 40 {
		httpJSONError(w, "InfoHash not valid", http.StatusInternalServerError)
		return t
	}
	t, tOk = torrentCli.Torrent(metainfo.NewHashFromHex(infoHash))
	if !tOk {
		httpJSONError(w, "Torrent not found", http.StatusNotFound)
	}
	getInfo(t)
	return t
}

func parseTorrentStats(t *torrent.Torrent) torrentStatsRes {
	var tsRes torrentStatsRes

	tsRes.InfoHash = t.InfoHash().String()
	tsRes.Name = t.Name()
	tsRes.TotalPeers = t.Stats().TotalPeers
	tsRes.ActivePeers = t.Stats().ActivePeers
	tsRes.HalfOpenPeers = t.Stats().HalfOpenPeers
	tsRes.PendingPeers = t.Stats().PendingPeers

	if t.Info() == nil {
		return tsRes
	}

	for _, tFile := range t.Files() {
		tsRes.Files.OnTorrent = append(tsRes.Files.OnTorrent, torrentStatsFilesOnTorrent{
			FileName:      tFile.DisplayPath(),
			FileSizeBytes: int(tFile.Length()),
		})
		if tFile.BytesCompleted() != 0 {
			tsRes.Files.OnDisk = append(tsRes.Files.OnDisk, torrentStatsFilesOnDisk{
				FileName:        tFile.DisplayPath(),
				StreamURL:       makePlayStreamURL(t.InfoHash().String(), tFile.DisplayPath(), true),
				BytesDownloaded: int(tFile.BytesCompleted()),
				FileSizeBytes:   int(tFile.Length()),
			})
		}
	}

	return tsRes
}
