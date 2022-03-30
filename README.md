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
go build streamrest.go
```

## Starting
`streamrest [-l PORT] [-d DOWNLOADDIR] [--noup]`

## Endpoints

### Add Magnet `(POST)`
`/api/addmagnet`

Add a torrent to the server

**Request body**
```
{
    Magnet: "magnetlink",
    Files: arrayofpreselectedfiles[]
}
```

### Select file `(POST)`
`/api/selectfile`

Initialize file for download

**Request body**
```
{
    InfoHash: "infohash"
    Files: arrayoffilenames[]
    AllFiles: false (Set true to download all files)
}
```

### Stream file `(GET)`
`/api/stream`

Streams a file from the **(select a file before streaming)**

```
/api/stream?infohash="infohash"&file="filename"
```

### Remove torrent `(DELETE)`
`/api/removetorrent`

Stops torrent download and deletes its files

**Request body**
```
{
    InfoHash: "infohash"
}
```

### Get Playlist `(GET)`

Automatically create a playlist file for the selected files

`/api/playlist?infohash="infohash"&file="file1"&file="file2"`

### List all torrents `(GET)`
`/api/torrents`

A array of infohash of all active torrents

### Get Torrent info `(GET)`
`/api/torrent`

Get info about the torrent

```
/api/torrent?infohash="infohash"
```