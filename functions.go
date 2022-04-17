package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/anacrolix/torrent"
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

func getTorrentFile(files []*torrent.File, filename string, exactName bool) *torrent.File {
	var tFile *torrent.File = nil
	for _, file := range files {
		if exactName && file.DisplayPath() == filename {
			tFile = file
		}
		if strings.Contains(strings.ToLower(file.DisplayPath()), strings.ToLower(filename)) {
			tFile = file
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

func initMagnet(magnet string, alldn []string, alltr []string) (*torrent.Torrent, error) {
	magnetString := magnet
	for _, dn := range alldn {
		magnetString += "&dn=" + url.QueryEscape(dn)
	}
	for _, tr := range alltr {
		magnetString += "&tr=" + url.QueryEscape(tr)
	}
	t, err := torrentCli.AddMagnet(magnetString)
	return t, err
}
