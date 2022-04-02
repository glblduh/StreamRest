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
    Magnet: "magnetlink"
}
```

### Get Playlist `(GET)`

**This is also for starting a stream**

Automatically create a playlist file for the selected files

```
/api/play?infohash="infohash"&file="file1"&file="file2"
```

To stream all files of torrent

```
/api/play?infohash="infohash"&file="ALLFILES"
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

### List all torrents `(GET)`
`/api/torrents`

A array of infohash of all active torrents

### Get Torrent info `(GET)`
`/api/torrent`

Get info about the torrent

```
/api/torrent?infohash="infohash"
```