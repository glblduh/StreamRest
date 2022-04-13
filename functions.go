package main

import (
	"net/url"
	"strings"
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
