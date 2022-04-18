package main

type errorRes struct {
	Error string
}

type addMagnetBody struct {
	Magnet   string
	AllFiles bool
	Files    []string
}

type addMagnetRes struct {
	InfoHash    string
	Name        string
	PlaylistURL string
	Files       []addMagnetFiles
}

type addMagnetFiles struct {
	FileName      string
	StreamURL     string
	FileSizeBytes int
}

type removeTorrentBody struct {
	InfoHash []string
}

type removeTorrentRes struct {
	Torrents []removeTorrentResRemoved
}

type removeTorrentResRemoved struct {
	Name     string
	InfoHash string
	Status   string
}

type listTorrentsRes struct {
	Torrents []listTorrentNameInfoHash
}

type listTorrentNameInfoHash struct {
	Name     string
	InfoHash string
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
