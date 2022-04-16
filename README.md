# StreamRest
*Torrent streaming server controlled by REST API's*

## Docker
```
docker run -d \
--name streamrest \
-p 1010:1010 \
glbl/streamrest:latest
```

## Compiling
```
go mod download
go build -ldflags="-extldflags -static -w -s" -tags=nosqlite
```

## Starting
`streamrest [-l PORT] [-d DOWNLOADDIR] [--noup]`

## Endpoints

### Get Playlist

**This is also for starting a stream**

Automatically create a playlist file for the selected files

*For specific directory it is seperated by `/` like `directory/file`*

```
/api/play?infohash="infohash"&file="file1"&file="directory/file2"
```

To play a magnet link directly

```
/api/playmagnet?magnet="magnetlink"&file="file1"&file="directory/file2"
```

To stream all files of torrent

```
/api/play?infohash="infohash"

or

/api/play?magnet="magnetlink"
```

### Add Magnet
`/api/addmagnet`

Start a torrent download without opening a stream

If none of the parameters are given it will respond with the files inside the torrent. Which can be used re-calling the endpoint with the specified filenames.

*For specific directory it is seperated by `/` like `directory/file`*

**Request body**
```
{
    Magnet: "magnetlink"

    // Parameters
    Files: ["file1", "directory/file2"] // Download selected file/s.
    AllFiles: false // Set to true to download all files in the torrent
}
```

### Manual stream

**This only works if the given file is downloading or already downloaded**

```
/api/stream?infohash="infohash"&file="filename"
```

### Remove torrent
`/api/removetorrent`

Stops torrent download and deletes its files

**Request body**
```
{
    InfoHash: ["infohash", "infohash2"]
}
```

### List all torrents
`/api/torrents`

A array of infohash of all active torrents

### Get Torrent info
`/api/torrent`

Get info about the torrent

```
/api/torrent?infohash="infohash"
```