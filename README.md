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

### Add Magnet
`/api/addmagnet`

Add a torrent to the server

**Request body**
```
{
    Magnet: "magnetlink"

    // Below are optional parameters
    AllFiles: false // Set to true to download all files in torrent without opening a stream
    Files: ["file1", "file2"] // Download selected file/s without opening a stream
}
```

### Get Playlist

**This is also for starting a stream**

Automatically create a playlist file for the selected files

```
/api/play?infohash="infohash"&file="file1"&file="file2"
```

To stream all files of torrent

```
/api/play?infohash="infohash"&file="ALLFILES"
```

### Play magnet

This endpoint automatically adds the magnet and send a playlist of the files

```
/api/playmagnet?magnet="magnetlink"&file="file1"&file="file2"
```

### Manual stream

**This is not recommended because it doesn't call the download for the file and only supports one file**

```
/api/stream?infohash="infohash"&file="filename"
```

### Remove torrent
`/api/removetorrent`

Stops torrent download and deletes its files

**Request body**
```
{
    InfoHash: "infohash"
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