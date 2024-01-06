package telegram

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"marathon_procrastination_bot/internal/env"
)

func mustToken() string {
	t, has := os.LookupEnv(env.TELEGRAM_TOKEN)
	if !has {
		panic("not defined " + env.TELEGRAM_TOKEN)
	}
	return t
}

type Storage interface {
	AddUser(ctx context.Context, userID int64) error
	RemoveUser(ctx context.Context, userID int64) error
	NewUserActivity(ctx context.Context, userID int64, activity string) error
	DeleteUserActivity(ctx context.Context, userID int64, activity string) error
	AppendUserActivity(ctx context.Context, userID int64, activity string) error
	UserActivities(ctx context.Context, userID int64) (activities []string, _ error)
	UserStats(ctx context.Context, userID int64, activity string) (total uint64, current uint64, err error)
	RotateUserStats(ctx context.Context, userID int64) error
	SetUserRotateHour(ctx context.Context, userID int64, hour int32) error
	UsersForRotate(ctx context.Context, hour int32) (ids []int64, err error)
}

type Agent struct {
	bot     *bot.Bot
	storage Storage
}

func New(s Storage) (_ *Agent, err error) {
	agent := &Agent{
		storage: s,
	}
	agent.bot, err = bot.New(mustToken(),
		//bot.WithSkipGetMe(),
		bot.WithDefaultHandler(func(ctx context.Context, bot *bot.Bot, update *models.Update) {
			_, _ = agent.Handle(ctx, bot, update)
		}),
	)
	if err != nil {
		return nil, err
	}
	return agent, nil
}

func (a *Agent) Bot() *bot.Bot {
	return a.bot
}

func (a *Agent) Storage() Storage {
	return a.storage
}

func (a *Agent) Handle(ctx context.Context, b *bot.Bot, update *models.Update) (*models.Message, error) {
	const enterActivityName = "В ответном сообщении напиши название активности"
	if update.Message != nil {
		if update.Message.Text == "/start" {
			err := a.storage.AddUser(ctx, update.Message.From.ID)
			if err != nil {
				return b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text: fmt.Sprintf("Не удалось сохранить пользователя @%s: %v",
						update.Message.From.Username,
						err,
					),
					ReplyToMessageID: update.Message.ID,
				})
			}
			return b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text: fmt.Sprintf("Ок, теперь в нашем марафоне участвует @%s\n"+
					"Используй команду /post - чтобы записать активности",
					update.Message.From.Username,
				),
				ReplyToMessageID: update.Message.ID,
			})
		}
		if update.Message.Text == "/stop" {
			err := a.storage.RemoveUser(ctx, update.Message.From.ID)
			if err != nil {
				return b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text: fmt.Sprintf("Не удалось удалить пользователя @%s: %v",
						update.Message.From.Username,
						err,
					),
					ReplyToMessageID: update.Message.ID,
				})
			}
			return b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text: fmt.Sprintf("Ок, теперь @%s не участвует в марафонах\n"+
					"Используй команду /start - чтобы участвовать в марафонах",
					update.Message.From.Username,
				),
				ReplyToMessageID: update.Message.ID,
			})
		}
		if update.Message.Text == "/stats" {
			activities, err := a.storage.UserActivities(ctx, update.Message.From.ID)
			if err != nil {
				return b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text: fmt.Sprintf("Не удалось получить активности пользователя @%s: %v",
						update.Message.From.Username,
						err,
					),
					ReplyToMessageID: update.Message.ID,
				})
			}
			var builder strings.Builder
			for _, activity := range activities {
				total, current, err := a.storage.UserStats(ctx, update.Message.From.ID, activity)
				if err != nil {
					return b.SendMessage(ctx, &bot.SendMessageParams{
						ChatID: update.Message.Chat.ID,
						Text: fmt.Sprintf("Не удалось получить статистику активности %q пользователя @%s: %v",
							activity,
							update.Message.From.Username,
							err,
						),
						ReplyToMessageID: update.Message.ID,
					})
				}
				fmt.Fprintf(&builder, "\n- %q (%d+%d)", activity, total, current)
			}
			return b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:           update.Message.Chat.ID,
				Text:             fmt.Sprintf("Статистика активности пользователя @%s:", update.Message.From.Username) + builder.String(),
				ReplyToMessageID: update.Message.ID,
			})
		}
		if update.Message.Text == "/rotate" {
			err := a.storage.RotateUserStats(ctx, update.Message.From.ID)
			if err != nil {
				return b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text: fmt.Sprintf("Не удалось обновить статистику пользователя @%s: %v",
						update.Message.From.Username,
						err,
					),
					ReplyToMessageID: update.Message.ID,
				})
			}
			return b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text: fmt.Sprintf("Статистика пользователя @%s обновлена.\n"+
					"Cтартовал новый день - не забывай про свои активности!\n"+
					"Используй команду /post - чтобы записать активности",
					update.Message.From.Username,
				),
				ReplyToMessageID: update.Message.ID,
			})
		}
		if update.Message.Text == "/set_rotate_hour" {
			location := time.Unix(int64(update.Message.Date), 0).Hour() - time.Now().UTC().Hour()
			rows := make([][]models.InlineKeyboardButton, 0, 4)
			for i := 0; i < 4; i++ {
				row := make([]models.InlineKeyboardButton, 0, 6)
				for j := 0; j < 6; j++ {
					h := i*6 + j
					row = append(row, models.InlineKeyboardButton{
						Text: strconv.Itoa(h), CallbackData: "/set_rotate_hour " + strconv.Itoa((h-location+24)%24),
					})
				}
				rows = append(rows, row)
			}
			return b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "Выбери час ежедневной ротации статистики",
				ReplyMarkup: &models.InlineKeyboardMarkup{
					InlineKeyboard: rows,
				},
				ReplyToMessageID: update.Message.ID,
			})
		}
		if update.Message.Text == "/post" {
			activities, err := a.storage.UserActivities(ctx, update.Message.From.ID)
			if err != nil {
				return b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text: fmt.Sprintf("Не удалось получить список активностей пользователя @%s: %v\n"+
						"Используй команду /start - чтобы участвовать в марафонах",
						update.Message.From.Username,
						err,
					),
					ReplyToMessageID: update.Message.ID,
				})
			}
			keyboard := &models.InlineKeyboardMarkup{
				InlineKeyboard: make([][]models.InlineKeyboardButton, 0, len(activities)+1),
			}
			for _, activity := range activities {
				keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, []models.InlineKeyboardButton{
					{Text: activity + "+1", CallbackData: "/post " + activity},
				})
			}
			keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, []models.InlineKeyboardButton{
				{Text: "Новый марафон", CallbackData: "/add"},
			})
			return b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:                   update.Message.Chat.ID,
				Text:                     "Записать активность",
				AllowSendingWithoutReply: true,
				ReplyMarkup:              keyboard,
				ReplyToMessageID:         update.Message.ID,
			})
		}
		if strings.HasPrefix(update.Message.Text, "/remove ") {
			activity := strings.TrimLeft(update.Message.Text, "/remove ")
			err := a.storage.DeleteUserActivity(ctx, update.Message.From.ID, activity)
			if err != nil {
				return b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text: fmt.Sprintf("Не удалось удалить активность %q пользователя @%s: %v",
						activity,
						update.Message.From.Username,
						err,
					),
					ReplyToMessageID: update.Message.ID,
				})
			}
			return b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text: fmt.Sprintf("Ок, теперь @%s больше не участвует в активности %q\n"+
					"Используй команду /stats - чтобы посмотреть статистику активностей",
					update.Message.From.Username,
					activity,
				),
				ReplyToMessageID: update.Message.ID,
			})
		}
		if update.Message.ReplyToMessage != nil {
			if update.Message.ReplyToMessage.Text == enterActivityName {
				activity := update.Message.Text
				err := a.storage.NewUserActivity(ctx, update.Message.From.ID, activity)
				if err != nil {
					return b.SendMessage(ctx, &bot.SendMessageParams{
						ChatID: update.Message.Chat.ID,
						Text: fmt.Sprintf("Не удалось сохранить активность %q пользователя @%s: %v",
							activity,
							update.Message.From.Username,
							err,
						),
						ReplyToMessageID: update.Message.ID,
					})
				}
				return b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text: fmt.Sprintf("Ок, теперь @%s участвует в активности %q\n"+
						"Используй команду /post - чтобы записать активность",
						update.Message.From.Username,
						activity,
					),
					ReplyToMessageID: update.Message.ID,
				})
			}
		}
	}
	if update.CallbackQuery != nil {
		if update.CallbackQuery.Data == "/add" {
			return b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:           update.CallbackQuery.Message.Chat.ID,
				Text:             enterActivityName,
				ReplyToMessageID: update.CallbackQuery.Message.ID,
			})
		}
		if strings.HasPrefix(update.CallbackQuery.Data, "/post ") {
			activity := strings.TrimLeft(update.CallbackQuery.Data, "/post ")
			if err := a.storage.AppendUserActivity(ctx, update.CallbackQuery.Sender.ID, activity); err != nil {
				return b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.CallbackQuery.Message.Chat.ID,
					Text: fmt.Sprintf("Не удалось сохранить активность %q пользователя @%s: %v",
						activity,
						update.CallbackQuery.Sender.Username,
						err,
					),
					ReplyToMessageID: update.CallbackQuery.Message.ID,
				})
			}
			return b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.CallbackQuery.Message.Chat.ID,
				Text: fmt.Sprintf("Активность %q успешна сохранена для @%s\n"+
					"Используй команду /stats - чтобы посмотреть статистику активностей",
					activity,
					update.CallbackQuery.Sender.Username,
				),
				ReplyToMessageID: update.CallbackQuery.Message.ID,
			})
		}
		if strings.HasPrefix(update.CallbackQuery.Data, "/set_rotate_hour ") {
			hour, err := strconv.Atoi(strings.TrimPrefix(update.CallbackQuery.Data, "/set_rotate_hour "))
			if err != nil {
				return b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.CallbackQuery.Message.Chat.ID,
					Text: fmt.Sprintf("Не удалось распарсить параметр %q команды /set_rotate_hour в число: %v",
						strings.TrimPrefix(update.CallbackQuery.Data, "/set_rotate_hour "),
						err,
					),
					ReplyToMessageID: update.CallbackQuery.Message.ID,
				})
			}
			if hour < 0 || hour > 23 {
				return b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.CallbackQuery.Message.Chat.ID,
					Text: fmt.Sprintf("Недопустимое значение параметра %d.\n"+
						"Параметр команды /set_rotate_hour должен быть числом от 0 до 23",
						hour,
					),
					ReplyToMessageID: update.CallbackQuery.Message.ID,
				})
			}
			if err := a.storage.SetUserRotateHour(ctx, update.CallbackQuery.Sender.ID, int32(hour)); err != nil {
				return b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.CallbackQuery.Message.Chat.ID,
					Text: fmt.Sprintf("Не удалось установить время ежедневной ротации статистики пользователя @%s: %v",
						update.CallbackQuery.Sender.Username,
						err,
					),
					ReplyToMessageID: update.CallbackQuery.Message.ID,
				})
			}
			return b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.CallbackQuery.Message.Chat.ID,
				Text: fmt.Sprintf("Время ежедневной ротации статистики пользователя @%s установлено в %d:00 UTC",
					update.CallbackQuery.Sender.Username,
					hour,
				),
				ReplyToMessageID: update.CallbackQuery.Message.ID,
			})
		}
	}
	return nil, nil
}
