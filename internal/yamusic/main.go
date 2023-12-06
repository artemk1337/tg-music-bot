package yamusic

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/ndrewnee/go-yamusic/yamusic"
	"github.com/rubyist/circuitbreaker"

	"tg_music_bot/internal/models"
)

type Service interface {
	Start(ctx context.Context)
	GetTracksByQuery(ctx context.Context, query string, limit int) (tracks []models.Track, err error)
}

type service struct {
	yaToken string
	timeout time.Duration

	client *yamusic.Client
}

func (s *service) Start(ctx context.Context) {
	circuitClient := circuit.NewHTTPClient(s.timeout, 10, nil)
	s.client = yamusic.NewClient(
		// if you want http client with circuit breaker
		yamusic.HTTPClient(circuitClient),
		// provide user_id and access_token (needed by some methods)
		yamusic.AccessToken(0, s.yaToken),
	)

	// set user id
	status, _, _ := s.client.Account().GetStatus(ctx)
	s.client.SetUserID(status.Result.Account.UID)
}

func (s *service) GetTracksByQuery(ctx context.Context, query string, limit int) (tracks []models.Track, err error) {
	return s.search(ctx, query, limit)
}

func (s *service) search(ctx context.Context, query string, limit int) (tracks []models.Track, err error) {
	res, resp, err := s.client.Search().Tracks(ctx, query, &yamusic.SearchOptions{
		Page:      0,
		NoCorrect: false,
	})
	if err != nil {
		return
	}
	if resp.StatusCode != 200 {
		return tracks, errors.New("status is not 200")
	}
	if res.Result.Tracks.Total == 0 {
		return tracks, nil
	}

	tracks = make([]models.Track, min(len(res.Result.Tracks.Results), limit))
	for i, track := range res.Result.Tracks.Results {
		if i >= limit {
			break
		}

		url, err := s.getTrackUrl(ctx, track.ID, true)
		if err != nil {
			return tracks, err
		}

		bfArtists := bytes.Buffer{}
		for i, artist := range track.Artists {
			bfArtists.WriteString(artist.Name)
			if i < len(track.Artists)-1 {
				bfArtists.WriteString(", ")
			}
		}

		tracks[i] = models.Track{
			Artists: bfArtists.String(),
			Title:   track.Title,
			Url:     url,
		}
	}

	return
}

func (s *service) getTrackUrl(ctx context.Context, trackId int, maxQuality bool) (url string, err error) {
	// get track info
	trackInfo, resp, err := s.client.Tracks().GetDownloadInfoResp(ctx, trackId)
	if err != nil {
		return
	}
	if resp.StatusCode != 200 {
		return url, errors.New("status is not 200")
	}

	if len(trackInfo.Result) == 0 {
		return url, errors.New("song url not exist")
	}

	// select quality
	maxBitraitIndex := 0
	if maxQuality {
		for i := 1; i < len(trackInfo.Result); i++ {
			if trackInfo.Result[i].BitrateInKbps > trackInfo.Result[maxBitraitIndex].BitrateInKbps {
				maxBitraitIndex = i
			}
		}
	}

	// get xml info
	req, err := s.client.NewRequest(http.MethodGet, trackInfo.Result[maxBitraitIndex].DownloadInfoURL, nil)
	if err != nil {
		return
	}

	dlInfo := new(yamusic.DownloadInfo)
	resp, err = s.client.Do(ctx, req, dlInfo)
	if err != nil {
		return
	}

	// a bit of magic
	const signPrefix = "XGRlBW9FXlekgbPrRHuSiA"
	sign := md5.Sum([]byte(signPrefix + dlInfo.Path[1:] + dlInfo.S))
	url = fmt.Sprintf(
		"https://%s/get-mp3/%s/%s%s",
		dlInfo.Host,
		hex.EncodeToString(sign[:]),
		dlInfo.TS, dlInfo.Path,
	)

	return
}

func NewService(yaToken string) Service {
	return &service{
		yaToken: yaToken,
		timeout: 5 * time.Second,
	}
}
