package main

import (
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
	var amBody addMagnetBody
	var amRes addMagnetRes

	if parseRequestBody(w, r, &amBody) != nil {
		return
	}

	if amBody.Magnet == "" {
		httpJSONError(w, "Magnet link is not provided", http.StatusNotFound)
		return
	}

	t, magErr := torrentCli.AddMagnet(amBody.Magnet)
	if magErr != nil {
		httpJSONError(w, "AddMagnet error", http.StatusInternalServerError)
		return
	}
	<-t.GotInfo()

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
	infoHash, ihok := r.URL.Query()["infohash"]
	fileName, fnok := r.URL.Query()["file"]

	if !ihok || !fnok {
		httpJSONError(w, "InfoHash or File is not provided", http.StatusNotFound)
		return
	}

	if len(infoHash[0]) != 40 {
		httpJSONError(w, "InfoHash is not valid", http.StatusInternalServerError)
		return
	}
	t, tok := torrentCli.Torrent(metainfo.NewHashFromHex(infoHash[0]))

	if !tok {
		httpJSONError(w, "Torrent not found", http.StatusNotFound)
		return
	}

	tFile := getTorrentFile(t.Files(), fileName[0], true)
	if tFile == nil {
		httpJSONError(w, "File not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename=\""+safenDisplayPath(tFile.DisplayPath())+"\"")
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
		httpJSONError(w, "No InfoHash provided", http.StatusNotFound)
		return
	}

	for i, curih := range rtBody.InfoHash {
		rtRes.Torrent = append(rtRes.Torrent, removeTorrentResRemoved{})
		if len(curih) != 40 {
			rtRes.Torrent[i] = removeTorrentResRemoved{
				Status: "INVALIDINFOHASH",
			}
			continue
		}

		t, tok := torrentCli.Torrent(metainfo.NewHashFromHex(curih))

		if !tok {
			rtRes.Torrent[i] = removeTorrentResRemoved{
				Status: "TORRENTNOTFOUND",
			}
			continue
		}

		t.Drop()
		rtRes.Torrent[i] = removeTorrentResRemoved{
			Name:     t.Name(),
			InfoHash: t.InfoHash().String(),
			Status:   "REMOVED",
		}
		if os.RemoveAll(filepath.Join(tcliConfs.DataDir, t.Name())) != nil {
			rtRes.Torrent[i] = removeTorrentResRemoved{
				Status: "FILEREMOVALERROR",
			}
		}
	}

	makeJSONResponse(w, &rtRes)
}

func listTorrents(w http.ResponseWriter, r *http.Request) {
	var ltRes listTorrentsRes

	for _, t := range torrentCli.Torrents() {
		ltRes.Torrents = append(ltRes.Torrents, listTorrentNameInfoHash{
			Name:     t.Name(),
			InfoHash: t.InfoHash().String(),
		})
	}

	if len(ltRes.Torrents) < 1 {
		w.WriteHeader(404)
	}

	makeJSONResponse(w, &ltRes)
}

func torrentStats(w http.ResponseWriter, r *http.Request) {
	infoHash, ihok := r.URL.Query()["infohash"]
	var tsRes torrentStatsRes

	if !ihok {
		httpJSONError(w, "InfoHash is not provided", http.StatusNotFound)
		return
	}

	if len(infoHash[0]) != 40 {
		httpJSONError(w, "InfoHash is not valid", http.StatusInternalServerError)
		return
	}
	t, tok := torrentCli.Torrent(metainfo.NewHashFromHex(infoHash[0]))

	if !tok {
		httpJSONError(w, "Torrent is not found", http.StatusNotFound)
		return
	}

	tsRes.InfoHash = t.InfoHash().String()
	tsRes.Name = t.Name()
	tsRes.TotalPeers = t.Stats().TotalPeers
	tsRes.ActivePeers = t.Stats().ActivePeers
	tsRes.HalfOpenPeers = t.Stats().HalfOpenPeers
	tsRes.PendingPeers = t.Stats().PendingPeers

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

	makeJSONResponse(w, &tsRes)
}

func playMagnet(w http.ResponseWriter, r *http.Request) {
	infoHash, ihOk := r.URL.Query()["infohash"]
	magnet, magOk := r.URL.Query()["magnet"]
	displayName, dnOk := r.URL.Query()["dn"]
	trackers, trOk := r.URL.Query()["tr"]
	files, fOk := r.URL.Query()["file"]

	if !magOk && !ihOk {
		httpJSONError(w, "Magnet link or InfoHash is not provided", http.StatusNotFound)
		return
	}

	var t *torrent.Torrent

	if magOk && !dnOk && !trOk && !ihOk {
		var magErr error
		t, magErr = torrentCli.AddMagnet(magnet[0])
		if magErr != nil {
			httpJSONError(w, "AddMagnet error", http.StatusInternalServerError)
			return
		}
	}

	if ihOk && !magOk && !dnOk && !trOk {
		if len(infoHash[0]) != 40 {
			httpJSONError(w, "InfoHash is not valid", http.StatusInternalServerError)
			return
		}

		var tOk bool
		t, tOk = torrentCli.Torrent(metainfo.NewHashFromHex(infoHash[0]))

		if !tOk {
			httpJSONError(w, "Torrent is not found", http.StatusNotFound)
			return
		}
	}

	if magOk && !ihOk && dnOk || trOk {
		torrentSpec := torrent.TorrentSpec{}

		magnetSplit := strings.Split(magnet[0], ":")
		torrentSpec.InfoHash = metainfo.NewHashFromHex(magnetSplit[len(magnetSplit)-1])

		if dnOk {
			torrentSpec.DisplayName = displayName[0]
		}

		if trOk {
			for i, tracker := range trackers {
				torrentSpec.Trackers = append(torrentSpec.Trackers, []string{})
				torrentSpec.Trackers[i] = append(torrentSpec.Trackers[i], tracker)
			}
		}

		var atsErr error
		t, _, atsErr = torrentCli.AddTorrentSpec(&torrentSpec)
		if atsErr != nil {
			httpJSONError(w, "AddTorrentSpec error", http.StatusInternalServerError)
			return
		}
	}

	<-t.GotInfo()

	w.Header().Set("Content-Disposition", "attachment; filename=\""+t.InfoHash().String()+".m3u\"")
	playList := "#EXTM3U\n"
	plCompleted := false

	httpScheme := "http"
	if r.Header.Get("X-Forwarded-Proto") != "" {
		httpScheme = r.Header.Get("X-Forwarded-Proto")
	}

	if !plCompleted && !t.Info().IsDir() {
		tFile := t.Files()[0]
		tFile.Download()
		playList += appendFilePlaylist(httpScheme, r.Host, t.InfoHash().String(), tFile.DisplayPath())
		plCompleted = true
	}

	if !plCompleted && t.Info().IsDir() && !fOk {
		t.DownloadAll()
		for _, tFile := range t.Files() {
			playList += appendFilePlaylist(httpScheme, r.Host, t.InfoHash().String(), tFile.DisplayPath())
		}
		plCompleted = true
	}

	if !plCompleted {
		for _, file := range files {
			tFile := getTorrentFile(t.Files(), file, false)
			if tFile == nil {
				continue
			}
			playList += appendFilePlaylist(httpScheme, r.Host, t.InfoHash().String(), tFile.DisplayPath())
			tFile.Download()
		}
		plCompleted = true
	}
	w.Write([]byte(playList))
}
