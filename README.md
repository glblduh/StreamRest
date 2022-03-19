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

### Select file `(POST)`
`/api/selectfile`

Initialize file for download

**Request body**
```
{
    InfoHash: "infohash"
    FileName: "filename"
}
```

### Stream file `(GET)`
`/api/stream`

Streams a file from the **(select a file before streaming)**

```
/api/stream?infohash="infohash"&filename="filename"
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

### Get Torrent info `(POST)`
`/api/torrent`

Get info about the torrent

**Request body**
```
{
    InfoHash: "infohash"
}
```