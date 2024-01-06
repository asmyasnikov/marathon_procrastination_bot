package main

import (
	"context"
	"marathon_procrastination_bot/internal/storage"
	"marathon_procrastination_bot/internal/telegram"
	"os"
	"os/signal"
	"time"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	s, err := storage.New(ctx)
	if err != nil {
		panic(err)
	}

	if err := s.UpdateSchema(); err != nil {
		panic(err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(-time.Since(time.Unix(int64(time.Now().UnixMilli()/1000/60/60+1)*60*60, 0))):
				func() {
					ids, err := s.UsersForRotate(ctx, int32(time.Unix(int64(time.Now().UnixMilli()/1000/60/60)*60*60, 0).UTC().Hour()))
					if err != nil {
						return
					}
					for _, id := range ids {
						if err := s.RotateUserStats(ctx, id); err != nil {
							return
						}
					}
				}()
			}
		}
	}()

	agent, err := telegram.New(s)
	if err != nil {
		panic(err)
	}

	agent.Bot().Start(ctx)
}
