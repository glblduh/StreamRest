package main

import (
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
	for _, file := range files {
		if exactName && strings.Compare(file.DisplayPath(), filename) == 0 {
			return file
		}
		if strings.Contains(strings.ToLower(file.DisplayPath()), strings.ToLower(filename)) {
			return file
		}
	}
	return nil
}
