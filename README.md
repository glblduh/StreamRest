# StreamRest
*Torrent streaming server controlled by REST API's*

## Compiling
```
go mod download
go build streamrest.go
```

## Starting
`streamrest [-l PORT] [-d DOWNLOADDIR] [--noup]`

## Endpoints
The default HTTP port is `1010`

### Add Magnet
`/api/addmagnet`
```
{
    Magnet: "magnetlink"
}
```

### Stream file
`/api/stream`

*The stream URL is using queries rather than JSON.*
The keys are `infohash` and `filename`.

### Remove torrent
`/api/removetorrent`
```
{
    InfoHash: "infohash"
}
```

### List all torrents
`/api/torrents`

### Get Torrent info
`/api/torrent`
```
{
    InfoHash: "infohash"
}
```