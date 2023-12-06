package tg

import (
	"io"
	"net/http"

	"tg_music_bot/internal/models"
)

func asyncTrackDownloader(
	readChn <-chan models.Track,
	writeChn chan<- models.Track,
) (err error) {
	for {
		select {
		case track, ok := <-readChn:
			if !ok {
				return
			}

			if track.Data, err = downloadFileByUrl(track.Url); err != nil {
				return err
			}

			writeChn <- track
		}
	}
}

func downloadFileByUrl(url string) (res []byte, err error) {
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}
