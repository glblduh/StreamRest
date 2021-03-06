package main

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
)

func addMagnet(w http.ResponseWriter, r *http.Request) {
	var amBody addMagnetBody
	var amRes addMagnetRes

	if parseRequestBody(w, r, &amBody) != nil {
		return
	}

	if amBody.Magnet == "" {
		httpJSONError(w, "Magnet link is not provided", http.StatusNotFound)
		return
	}

	t := initMagnet(w, amBody.Magnet, []string{}, []string{})
	if t == nil {
		return
	}

	amRes.InfoHash = t.InfoHash().String()
	amRes.Name = t.Name()

	if amBody.AllFiles {
		t.DownloadAll()
		amRes.PlaylistURL = makePlayStreamURL(t.InfoHash().String(), "", false)
		for _, tFile := range t.Files() {
			amRes.Files = append(amRes.Files, addMagnetFiles{
				FileName:      tFile.DisplayPath(),
				StreamURL:     makePlayStreamURL(t.InfoHash().String(), tFile.DisplayPath(), true),
				FileSizeBytes: int(tFile.Length()),
			})
		}
		makeJSONResponse(w, &amRes)
		return
	}

	if len(amBody.Files) > 0 {
		amRes.PlaylistURL = makePlayStreamURL(t.InfoHash().String(), "", false)
		for _, selFile := range amBody.Files {
			tFile := getTorrentFile(t.Files(), selFile, false)
			if tFile == nil {
				continue
			}
			tFile.Download()
			amRes.PlaylistURL += "&file=" + url.QueryEscape(tFile.DisplayPath())
			amRes.Files = append(amRes.Files, addMagnetFiles{
				FileName:      tFile.DisplayPath(),
				StreamURL:     makePlayStreamURL(t.InfoHash().String(), tFile.DisplayPath(), true),
				FileSizeBytes: int(tFile.Length()),
			})
		}
		makeJSONResponse(w, &amRes)
		return
	}

	for _, tFile := range t.Files() {
		amRes.Files = append(amRes.Files, addMagnetFiles{
			FileName:      tFile.DisplayPath(),
			FileSizeBytes: int(tFile.Length()),
		})
	}
	makeJSONResponse(w, &amRes)
}

func beginStream(w http.ResponseWriter, r *http.Request) {
	infoHash, ihOk := r.URL.Query()["infohash"]
	fileName, fnOk := r.URL.Query()["file"]

	if !ihOk || !fnOk {
		httpJSONError(w, "InfoHash or File is not provided", http.StatusNotFound)
		return
	}

	t := getTorrent(w, infoHash[0])
	if t == nil {
		return
	}

	tFile := getTorrentFile(t.Files(), fileName[0], true)
	if tFile == nil {
		httpJSONError(w, "File not found", http.StatusNotFound)
		return
	}
	fileRead := tFile.NewReader()
	defer fileRead.Close()
	fileRead.SetReadahead(tFile.Length() / 100)
	http.ServeContent(w, r, tFile.DisplayPath(), time.Now(), fileRead)
}

func removeTorrent(w http.ResponseWriter, r *http.Request) {
	var rtBody removeTorrentBody
	var rtRes removeTorrentRes

	if parseRequestBody(w, r, &rtBody) != nil {
		return
	}

	if len(rtBody.InfoHash) < 1 {
		httpJSONError(w, "InfoHash is not provided", http.StatusNotFound)
		return
	}

	for i, curih := range rtBody.InfoHash {
		rtRes.Torrents = append(rtRes.Torrents, removeTorrentResRemoved{})
		if len(curih) != 40 {
			rtRes.Torrents[i] = removeTorrentResRemoved{
				Status: "INVALIDINFOHASH",
			}
			continue
		}

		t, tok := torrentCli.Torrent(metainfo.NewHashFromHex(curih))

		if !tok {
			rtRes.Torrents[i] = removeTorrentResRemoved{
				Status: "TORRENTNOTFOUND",
			}
			continue
		}

		t.Drop()
		rtRes.Torrents[i] = removeTorrentResRemoved{
			Name:     t.Name(),
			InfoHash: t.InfoHash().String(),
			Status:   "REMOVED",
		}
		if os.RemoveAll(filepath.Join(tcliConfs.DataDir, t.Name())) != nil {
			rtRes.Torrents[i] = removeTorrentResRemoved{
				Status: "FILEREMOVALERROR",
			}
		}
	}

	makeJSONResponse(w, &rtRes)
}

func listTorrents(w http.ResponseWriter, r *http.Request) {
	infoHash, ihOk := r.URL.Query()["infohash"]
	var ltRes listTorrentsRes

	allTorrents := torrentCli.Torrents()

	if ihOk {
		allTorrents = nil
		t := getTorrent(w, infoHash[0])
		if t == nil {
			return
		}
		allTorrents = append(allTorrents, t)
	}

	for _, t := range allTorrents {
		ltRes.Torrents = append(ltRes.Torrents, parseTorrentStats(t))
	}

	if !ihOk && len(ltRes.Torrents) < 1 {
		w.WriteHeader(404)
	}

	makeJSONResponse(w, &ltRes)
}

func playMagnet(w http.ResponseWriter, r *http.Request) {
	infoHash, ihOk := r.URL.Query()["infohash"]
	magnet, magOk := r.URL.Query()["magnet"]
	displayName, _ := r.URL.Query()["dn"]
	trackers, _ := r.URL.Query()["tr"]
	files, fOk := r.URL.Query()["file"]

	if !magOk && !ihOk {
		httpJSONError(w, "Magnet link or InfoHash is not provided", http.StatusNotFound)
		return
	}

	var t *torrent.Torrent
	if magOk && !ihOk {
		t = initMagnet(w, magnet[0], displayName, trackers)
	}

	if ihOk && !magOk {
		t = getTorrent(w, infoHash[0])
	}

	if t == nil {
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename=\""+t.InfoHash().String()+".m3u\"")
	playList := "#EXTM3U\n"

	httpScheme := "http"
	if r.Header.Get("X-Forwarded-Proto") != "" {
		httpScheme = r.Header.Get("X-Forwarded-Proto")
	}

	if !fOk {
		t.DownloadAll()
		for _, tFile := range t.Files() {
			playList += appendFilePlaylist(httpScheme, r.Host, t.InfoHash().String(), tFile.DisplayPath())
		}
	}

	for _, file := range files {
		tFile := getTorrentFile(t.Files(), file, false)
		if tFile == nil {
			continue
		}
		playList += appendFilePlaylist(httpScheme, r.Host, t.InfoHash().String(), tFile.DisplayPath())
		tFile.Download()
	}

	w.Write([]byte(playList))
}
