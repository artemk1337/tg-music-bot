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
	tgToken string `envconfig:"tg_token" required:"true"`
	yaToken string `envconfig:"YATOKEN" required:"true"`

	tracksLimit int  `envconfig:"TRACKS_LIMIT" default:"10"`
	cacheTTL    int  `envconfig:"CACHE_TTL" default:"60"` // minutes
	debug       bool `envconfig:"DEBUG" default:"false"`
}

func main() {
	var cfg configuration

	ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt)

	if err := envconfig.Process("", &cfg); err != nil {
		println(err)
		os.Exit(1)
	}

	yaClient := yamusic.NewService(cfg.yaToken)
	tgClient := tg.NewService(yaClient, cfg.tgToken, cfg.tracksLimit, cfg.cacheTTL, cfg.debug)

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
