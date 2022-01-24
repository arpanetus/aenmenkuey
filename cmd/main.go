package main

import (
	"github.com/arpanetus/aenmenkuey/pkg/parse"
	"log"
)

const (
	BaseUrl      = "https://qazradio.fm/kz/audiosloadmore"
	SaveFolder   = "/tmp/music"
	ErrorLogFile = "/tmp/music/err.log.jsonl"
)

func main() {
	p, err := parse.NewParse(BaseUrl, ErrorLogFile)
	if err != nil {
		log.Panic(err)
	}

	songsChan := make(chan *parse.Song, 20)
	stopChan := make(chan struct{}, 1)
	go p.Chain(songsChan, stopChan)

	p.DownloadFromChan(songsChan, stopChan, SaveFolder)
	p.WriteErrors()
}
