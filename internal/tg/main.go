package tg

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"tg_music_bot/internal/models"
)

type Service interface {
	Start(ctx context.Context)
}

type yaMusicService interface {
	GetTracksByQuery(ctx context.Context, query string, limit int) (tracks []models.Track, err error)
}

type service struct {
	tgToken string

	tracksLimit int
	cacheTTL    int // minutes
	debug       bool

	yaMusic yaMusicService
}

func (s *service) Start(ctx context.Context) {
	bot, err := tgbotapi.NewBotAPI(s.tgToken)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = s.debug

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			fmt.Println("stop tg bot")
			return
		case update := <-updates:
			if update.Message != nil { // If we got a message
				err = s.parseMessage(ctx, bot, update)
				if err != nil {
					log.Println(err)
				}
			} else if update.InlineQuery != nil {
				err = s.parseInlineQuery(ctx, bot, update)
				if err != nil {
					log.Println(err)
				}
			}
		}
	}
}

func (s *service) parseInlineQuery(ctx context.Context, bot *tgbotapi.BotAPI, update tgbotapi.Update) (err error) {
	tracks, err := s.searchTracksByQuery(ctx, update.InlineQuery.Query, s.tracksLimit, false)
	if err != nil {
		return
	}

	results := make([]interface{}, len(tracks))
	for i, track := range tracks {
		caption := fmt.Sprintf("%s - %s", track.Artists, track.Title)
		resultAudio := tgbotapi.NewInlineQueryResultAudio(strconv.Itoa(i), track.Url, caption)
		resultAudio.Title = track.Title
		resultAudio.Performer = track.Artists
		results[i] = resultAudio
	}

	inlineConf := tgbotapi.InlineConfig{
		InlineQueryID: update.InlineQuery.ID,
		IsPersonal:    true,
		CacheTime:     s.cacheTTL,
		Results:       results,
	}

	if _, err = bot.Request(inlineConf); err != nil {
		return
	}

	return
}

func (s *service) parseMessage(ctx context.Context, bot *tgbotapi.BotAPI, update tgbotapi.Update) (err error) {
	// TODO: add another cases to process message
	return s.sendAudioMessage(ctx, bot, update, s.tracksLimit)
}

func (s *service) sendAudioMessage(ctx context.Context, bot *tgbotapi.BotAPI, update tgbotapi.Update, limit int) (err error) {
	tracks, err := s.searchTracksByQuery(ctx, update.Message.Text, limit, false)
	if err != nil {
		return
	}

	writeChn := make(chan models.Track, limit)
	defer close(writeChn)
	readChn := make(chan models.Track, limit)
	defer close(readChn)

	go func() {
		err := asyncTrackDownloader(writeChn, readChn)
		if err != nil {
			log.Println(err)
			return
		}
	}()

	for _, track := range tracks {
		writeChn <- track
	}

	for range tracks {
		track := <-readChn

		caption := fmt.Sprintf("%s - %s", track.Artists, track.Title)
		file := tgbotapi.FileBytes{
			Name:  caption,
			Bytes: track.Data,
		}

		msg := tgbotapi.NewAudio(update.Message.Chat.ID, file)
		msg.Performer = track.Artists
		msg.Title = track.Title

		msg.ReplyToMessageID = update.Message.MessageID

		if _, err = bot.Send(msg); err != nil {
			return
		}
	}

	return
}

func (s *service) searchTracksByQuery(ctx context.Context, query string, limit int, withDownload bool) (tracks []models.Track, err error) {
	startTime := time.Now()

	tracks, err = s.yaMusic.GetTracksByQuery(ctx, query, limit)

	if withDownload {
		for i, track := range tracks {
			byteArray, err := downloadFileByUrl(track.Url)
			if err != nil {
				return tracks, err
			}

			tracks[i].Data = byteArray
		}
	}

	fmt.Println("Searching tracks time:", time.Now().Sub(startTime))
	return
}

func NewService(
	yaMusic yaMusicService,
	tgToken string,
	tracksLimit int,
	cacheTTL int,
	debug bool,
) Service {
	return &service{
		tgToken:     tgToken,
		yaMusic:     yaMusic,
		tracksLimit: tracksLimit,
		cacheTTL:    cacheTTL,
		debug:       debug,
	}
}
