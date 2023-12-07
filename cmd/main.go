package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/kelseyhightower/envconfig"

	"tg_music_bot/internal/tg"
	"tg_music_bot/internal/yamusic"
)

type configuration struct {
	TgToken string `envconfig:"TG_TOKEN" required:"true" default:"966549792:AAEW2fJH7DMRYydCpqDyfvJ6epyO1IGAmOk"`
	YaToken string `envconfig:"YA_TOKEN" required:"true" default:"AQAAAAAi5-PqAAG8Xi6q5GuvEU8Mn9BuFAycw2g"`

	TracksLimit int  `envconfig:"TRACKS_LIMIT" default:"10"`
	CacheTTL    int  `envconfig:"CACHE_TTL" default:"60"` // minutes
	Debug       bool `envconfig:"DEBUG" default:"false"`
}

func main() {
	var cfg configuration

	ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt)

	if err := envconfig.Process("tg_bot", &cfg); err != nil {
		println(err)
		os.Exit(1)
	}

	yaClient := yamusic.NewService(cfg.YaToken)
	tgClient := tg.NewService(yaClient, cfg.TgToken, cfg.TracksLimit, cfg.CacheTTL, cfg.Debug)

	wg := sync.WaitGroup{}
	startServices(ctx, &wg, tgClient, yaClient)

	wg.Wait()

	fmt.Println("finish app")
}

func startServices(ctx context.Context, wg *sync.WaitGroup, tgClient tg.Service, yaClient yamusic.Service) {
	yaClient.Start(ctx)
	go startWithWaitGroup(ctx, wg, tgClient.Start)

	time.Sleep(1 * time.Second)
}

func startWithWaitGroup(ctx context.Context, wg *sync.WaitGroup, f func(ctx context.Context)) {
	wg.Add(1)
	defer wg.Done()

	f(ctx)
}
