package telegram

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"marathon_procrastination_bot/internal/env"
)

const welcome = `

**Правила марафона!!!**

Заводишь себе ***Новый марафон*** (привычку, обязанность) через /post и **каждый** день выполняешь то, что задумал.  
При этом отписываешься в ботике, что выполнил.
Ботик считает непрерывное количество дней - это твоя ачивка (как у зависимых медальки "дней в завязке").
Если пропускаешь день - счетчик непрерывного количества дней сбрасывается в **НОЛЬ**. 
Так что не пропускай!!!

Удачи тебе, друг мой!
`

func mustToken() string {
	t, has := os.LookupEnv(env.TELEGRAM_TOKEN)
	if !has {
		panic("not defined " + env.TELEGRAM_TOKEN)
	}
	return t
}

type Storage interface {
	AddUser(ctx context.Context, userID int64, chatID int64) error
	RemoveUser(ctx context.Context, userID int64) error
	NewUserActivity(ctx context.Context, userID int64, activity string) error
	DeleteUserActivity(ctx context.Context, userID int64, activity string) error
	AppendUserActivity(ctx context.Context, userID int64, activity string) error
	UserActivities(ctx context.Context, userID int64) (activities []string, _ error)
	UserRegistrationChatID(ctx context.Context, userID int64) (chatID int64, _ error)
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

func (a *Agent) PingUser(ctx context.Context, userID int64) error {
	activities, err := a.storage.UserActivities(ctx, userID)
	if err != nil {
		return err
	}
	chatID, err := a.storage.UserRegistrationChatID(ctx, userID)
	if err != nil {
		return err
	}
	var builder strings.Builder
	for _, activity := range activities {
		total, current, err := a.storage.UserStats(ctx, userID, activity)
		if err != nil {
			return err
		}
		if current == 0 {
			_, _ = fmt.Fprintf(&builder, "\n- %q (%d+%d)", activity, total, current)
		}
	}
	_, err = a.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text: "Алло!!!\n" +
			"Кажется, ты забыл про свои марафоны:\n" + builder.String() + "\n\n" +
			"Используй команду /post - чтобы записать участие в марафоне",
	})
	if err != nil {
		return err
	}
	return err
}

func (a *Agent) Handle(ctx context.Context, b *bot.Bot, update *models.Update) (*models.Message, error) {
	const enterActivityName = "В ОТВЕТНОМ СООБЩЕНИИ (кнопка меню Ответить/Reply) напиши название марафона"
	if update.Message != nil {
		if update.Message.Text == "/start" {
			err := a.storage.AddUser(ctx, update.Message.From.ID, update.Message.Chat.ID)
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
				Text: fmt.Sprintf("Ок, теперь в нашем марафоне участвует @%s",
					update.Message.From.Username,
				) + welcome,
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
					Text: fmt.Sprintf("Не удалось получить марафоны пользователя @%s: %v",
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
						Text: fmt.Sprintf("Не удалось получить статистику марафона %q пользователя @%s: %v",
							activity,
							update.Message.From.Username,
							err,
						),
						ReplyToMessageID: update.Message.ID,
					})
				}
				_, _ = fmt.Fprintf(&builder, "\n- %q (%d+%d)", activity, total, current)
			}
			return b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:           update.Message.Chat.ID,
				Text:             fmt.Sprintf("Статистика марафонов пользователя @%s:", update.Message.From.Username) + builder.String(),
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
					"Cтартовал новый день - не забывай про свои марафоны!\n"+
					"Используй команду /post - чтобы записать участие в марафоне",
					update.Message.From.Username,
				),
				ReplyToMessageID: update.Message.ID,
			})
		}
		if update.Message.Text == "/set_rotate_hour" {
			rows := make([][]models.InlineKeyboardButton, 0, 4)
			for i := 0; i < 4; i++ {
				row := make([]models.InlineKeyboardButton, 0, 6)
				for j := 0; j < 6; j++ {
					h := i*6 + j
					row = append(row, models.InlineKeyboardButton{
						Text: strconv.Itoa(h), CallbackData: "/set_rotate_hour " + strconv.Itoa(h),
					})
				}
				rows = append(rows, row)
			}
			return b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "Выбери час ежедневной ротации статистики (UTC)",
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
					Text: fmt.Sprintf("Не удалось получить список марафонов пользователя @%s: %v\n"+
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
				Text:                     "Записать участие в марафоне",
				AllowSendingWithoutReply: true,
				ReplyMarkup:              keyboard,
				ReplyToMessageID:         update.Message.ID,
			})
		}
		if update.Message.Text == "/remove" {
			activities, err := a.storage.UserActivities(ctx, update.Message.From.ID)
			if err != nil {
				return b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text: fmt.Sprintf("Не удалось получить список марафонов пользователя @%s: %v\n"+
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
					{Text: activity, CallbackData: "/remove " + activity},
				})
			}
			return b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID:                   update.Message.Chat.ID,
				Text:                     "Больше не хочу участвовать в марафоне",
				AllowSendingWithoutReply: true,
				ReplyMarkup:              keyboard,
				ReplyToMessageID:         update.Message.ID,
			})
		}
		if update.Message.ReplyToMessage != nil {
			if update.Message.ReplyToMessage.Text == enterActivityName {
				activity := update.Message.Text
				err := a.storage.NewUserActivity(ctx, update.Message.From.ID, activity)
				if err != nil {
					return b.SendMessage(ctx, &bot.SendMessageParams{
						ChatID: update.Message.Chat.ID,
						Text: fmt.Sprintf("Не удалось сохранить участие в марафоне %q пользователя @%s: %v",
							activity,
							update.Message.From.Username,
							err,
						),
						ReplyToMessageID: update.Message.ID,
					})
				}
				return b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text: fmt.Sprintf("Ок, теперь @%s участвует в марафоне %q\n"+
						"Используй команду /post - чтобы записать участие в марафоне",
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
					Text: fmt.Sprintf("Не удалось сохранить участие в марафоне %q пользователя @%s: %v",
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
					"Используй команду /stats - чтобы посмотреть статистику марафонов",
					activity,
					update.CallbackQuery.Sender.Username,
				),
				ReplyToMessageID: update.CallbackQuery.Message.ID,
			})
		}
		if strings.HasPrefix(update.CallbackQuery.Data, "/remove ") {
			activity := strings.TrimLeft(update.CallbackQuery.Data, "/remove ")
			err := a.storage.DeleteUserActivity(ctx, update.CallbackQuery.Sender.ID, activity)
			if err != nil {
				return b.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: update.CallbackQuery.Message.Chat.ID,
					Text: fmt.Sprintf("Не удалось удалить марафон %q пользователя @%s: %v",
						activity,
						update.CallbackQuery.Sender.Username,
						err,
					),
					ReplyToMessageID: update.CallbackQuery.Message.ID,
				})
			}
			return b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.CallbackQuery.Message.Chat.ID,
				Text: fmt.Sprintf("Ок, теперь @%s больше не участвует в марафоне %q\n"+
					"Используй команду /stats - чтобы посмотреть статистику марафонов",
					update.CallbackQuery.Sender.Username,
					activity,
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
