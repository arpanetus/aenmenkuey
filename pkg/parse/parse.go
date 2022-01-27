package parse

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/arpanetus/aenmenkuey/pkg/util"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"
)

const (
	ten = 10
)

type Parse struct {
	c            *http.Client
	errorLogFile string
	baseURL      url.URL
	genRe        *regexp.Regexp
	linkRe       *regexp.Regexp
	titleRe      *regexp.Regexp
	errMutex     *sync.Mutex
	errors       []*ErrorLog
}

func NewParse(baseURL, errorLogFile string) (*Parse, error) {
	c := new(http.Client)

	c.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	c.Timeout = time.Second * 180

	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(`<a href=.+<i class="fa fa-download"><\/i><\/a><\/span>`)
	linkRe := regexp.MustCompile(`href=\"https://.+.mp3`)
	titleRe := regexp.MustCompile(`download=\".+\"><i `)

	return &Parse{
		c:            c,
		errorLogFile: errorLogFile,
		baseURL:      *u,
		genRe:        re,
		linkRe:       linkRe,
		titleRe:      titleRe,
		errMutex:     &sync.Mutex{},
		errors:       make([]*ErrorLog, 0),
	}, nil
}

func (p *Parse) Content(id int) (out *DefaultResponse, err error) {
	u := util.AppendTrailingSlash(p.baseURL.String()) + strconv.FormatInt(int64(id), ten)

	r, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	res, err := p.c.Do(r)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	decoder := json.NewDecoder(res.Body)
	if err = decoder.Decode(&out); err != nil {
		return nil, err
	}

	return out, nil
}

func (p *Parse) Songs(in *DefaultResponse) (songs []*Song, err error) {
	maybeSongs := p.genRe.FindAllString(in.Content, -1)
	for i := range maybeSongs {
		var link, title string

		maybeLinks := p.linkRe.FindAllString(maybeSongs[i], -1)
		if len(maybeLinks) != 0 {
			link = string([]rune(maybeLinks[0])[6:])
		}

		maybeTitles := p.titleRe.FindAllString(maybeSongs[i], -1)
		if len(maybeTitles) != 0 {
			titleToCleanse := []rune(maybeTitles[0])[10:]
			title = string(titleToCleanse[:len(titleToCleanse)-5])
		}

		song := &Song{
			Link:     link,
			Title:    title,
			RawValue: maybeSongs[i],
			IsParsed: link == "" || title == "",
		}

		songs = append(songs, song)
	}

	return
}

func (p *Parse) Chain(songsChan chan<- *Song, stopChan chan<- struct{}) {
	for i := 0; ; i++ {
		log.Printf("getting page by id{%d}", i)
		content, err := p.Content(i)
		if err != nil {
			log.Printf("failed to get page by id{%d}, since: %v", i, err)
		}
		if content.Content == "" {
			stopChan <- struct{}{}
			break
		}
		songs, err := p.Songs(content)
		for _, song := range songs {
			songsChan <- song
		}
	}
}

func (p *Parse) Download(
	wg *sync.WaitGroup,
	folder string,
	song *Song,
) {
	defer wg.Done()

	if song.Link == "" {
		a := fmt.Sprintf("cannot parse, thus not downloading: %s", song.RawValue)
		log.Println(a)

		p.appendIntoErrorLog(&ErrorLog{
			Song: song,
			Err:  a,
		})

		return
	}

	resp, err := p.c.Get(song.Link)
	if err != nil {
		a := fmt.Sprintf("cannot download: %v", err)
		log.Println(a)
		p.appendIntoErrorLog(&ErrorLog{
			Song: song,
			Err:  a,
		})
		return
	}
	defer resp.Body.Close()

	f, err := os.Create(util.AppendTrailingSlash(folder) + song.Title + ".mp3")
	if err != nil {
		a := fmt.Sprintf("cannot create file: %v", err)
		log.Println(a)
		p.appendIntoErrorLog(&ErrorLog{
			Song: song,
			Err:  a,
		})

		return
	}
	defer f.Close()

	if _, err = io.Copy(f, resp.Body); err != nil {
		a := fmt.Sprintf("cannot copy from resp to file: %v", err)
		log.Println(a)
		p.appendIntoErrorLog(&ErrorLog{
			Song: song,
			Err:  a,
		})

		return
	}
}

func (p *Parse) appendIntoErrorLog(errorLog *ErrorLog) {
	p.errMutex.Lock()
	p.errors = append(p.errors, errorLog)
	p.errMutex.Unlock()
}

func (p *Parse) DownloadFromChan(
	songsChan <-chan *Song,
	stopChan <-chan struct{},
	folder string,
) {
	wg := &sync.WaitGroup{}

	for {
		select {
		case song := <-songsChan:
			log.Printf("\nTitle: %s[%t]\nLink: %s\n", song.Title, song.IsParsed, song.Link)
			wg.Add(1)
			go p.Download(wg, folder, song)
		case <-stopChan:
			goto exitFromLoop
		}
	}

exitFromLoop:
	log.Printf("called all gorotines")
	wg.Wait()

	log.Printf("downloaded all possible files")

}

func (p *Parse) WriteErrors() {
	ef, err := os.Create(p.errorLogFile)
	if err != nil {
		log.Printf("=== THIS IS FUCKED UP I CAN'T CREATE THE ERROR LOG FILE DUDE ===\n%v", err)

		return
	}

	defer ef.Close()

	for _, e := range p.errors {
		eb, err := json.Marshal(e)
		if err != nil {
			log.Printf("cannot marshal: %v", err)

			continue
		}

		eb = append(eb, []byte("\n")...)
		_, err = ef.Write(eb)
		if err != nil {
			log.Printf("cannot write bytes: %v", err)
		}
	}
}
