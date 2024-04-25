package discord

import (
	"sync"
	"time"

	"github.com/zatxm/any-proxy/internal/config"
	"github.com/zatxm/fhblade"
	"go.uber.org/zap"
)

type LiveDiscordBot interface {
	Close()
}

type liveDiscordBot struct {
	sync.Mutex
	quit chan struct{}
}

func NewLiveDiscordBot() LiveDiscordBot {
	l := &liveDiscordBot{}
	l.quit = make(chan struct{})
	go l.run()
	return l
}

func (l *liveDiscordBot) run() {
	// 创建定时器，设置定时器的间隔为距离明天凌晨的时间间隔
	now := time.Now()
	durationUntilTomorrow := time.Until(time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location()))
	t := time.NewTimer(durationUntilTomorrow)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			l.Lock()

			botIds := config.V().Coze.Discord.CozeBot
			if len(botIds) > 0 {
				for k := range botIds {
					botId := botIds[k]
					_, err := SendMessage("Hi!", botId)
					if err != nil {
						fhblade.Log.Error("Active bot error",
							zap.Error(err),
							zap.String("botId", botId))
					} else {
						fhblade.Log.Debug("Active bot success", zap.String("botId", botId))
					}
				}
			}

			l.Unlock()
			t.Reset(24 * time.Hour)
		case <-l.quit:
			return
		}
	}
}

func (l *liveDiscordBot) Close() {
	close(l.quit)
}
