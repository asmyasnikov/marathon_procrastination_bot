# marathon_procrastination_bot

## Запуск

Запускается из Яндекс Функций, указав точку входа - `index.Handler`

### env-переменные

* `TELEGRAM_TOKEN` - токен бота, полученный от BotFather
* `YDB_CONNECTION_STRING` - строка подключения к YDB

### дополнительные env-переменные для локального запуска

* `YDB_ANONYMOUS_CREDENTIALS` - использовать анонимную аутентификацию в YDB
* `YDB_METADATA_CREDENTIALS` - использовать аутентификацию в YDB с помощью сервиса метаданных
* `YDB_SERVICE_ACCOUNT_KEY_FILE_CREDENTIALS` - использовать аутентификацию в YDB с помощью файла сервисного аккаунта
* `YDB_ACCESS_TOKEN_CREDENTIALS` - использовать аутентификацию в YDB с помощью токена

### Авторизация в телеграме

Обеспечивается через токен `TELEGRAM_TOKEN`, полученный от BotFather

### Авторизация в YDB

Используется [ydb-go-sdk-auth-environ](https://github.com/ydb-platform/ydb-go-sdk-auth-environ)

Можно ничего не делать, если запускать на виртуалке в Облаке и привязать к ней сервисный аккаунт с правами доступа к базе данных

## Варианты запуска

### Как сервис

Бот запускается в режиме поллинга. 
Для запуска следует указать необходимые переменные окруженния 

```shell
go run main.go
```

### Как serverless-функция в Облаке

Следует указать точку входа `index.Handler`, указав необходимые переменные окружения.
Также для корректной работы serverless-функции необходимо создать сервисный аккаунт с ролям:
* `ydb.viewer` - для выполнения `DQL` запросов
* `ydb.editor` - для выполнения `DML` и `DDL` запросов
* `serverless.functions.invoker` - для запуска функции
* `iam.serviceAccounts.user` - для деплоя через GitHub Actions
