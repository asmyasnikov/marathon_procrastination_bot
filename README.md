# marathon_procrastination_bot

## Запуск

Запускается из Яндекс Функций, указав точку входа - `index.Handler`

### Команды бота

Команды, которые сам бот предлагает использовать:
* `/start` - для начала работы с ботом (регистрация пользователя)
* `/post` - для записи ранее обозначенной активности или создания новой активности
* `/stats` - для просмотра статистики марафонов

Недокументированные команды:
* `/stop` - для завершения работы с ботом (удаление пользователя)
* `/rotate` - для принудительной ротации статистики дня
* `/remove <активность>` - для исключения активности из марафонов
* `/set_rotate_hour <час автоматической ротации>` - для установки часа автоматической ротации марафонов (по умолчанию - 00:00 UTC)

### env-переменные

* `TELEGRAM_TOKEN` - токен бота, полученный от BotFather
* `YDB_CONNECTION_STRING` - строка подключения к YDB
* `MAGIC_NUMBER` - специальный номер-маркер для админских запросов. По умолчанию равен 347863284

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
* `serverless.functions.admin` - для деплоя через GitHub Actions
* `iam.serviceAccounts.user` - для деплоя через GitHub Actions

#### Миграции схемы

Для применения миграций следует вызвать функцию с телом:
```json
{
  "migrate_schema": true
}
```

Например, так:
```shell
curl -X POST https://functions.yandexcloud.net/<function-id> 
   -H "Content-Type: application/json"
   -d '{"migrate_schema": true}'  
```

#### Триггер для пробуждения

Функцию можно вызвать для выполнения операций:
* применения миграций
    Для применения миграций следует вызвать функцию с телом:
    ```json
    {
      "magic_number": <MAGIC_NUMBER>,
      "migrate_schema": true
    }
    ```
    
    Например, так:
    ```shell
    curl -X POST https://functions.yandexcloud.net/<function-id> 
       -H "Content-Type: application/json"
       -d '{"magic_number":<MAGIC_NUMBER>,"migrate_schema":true}'  
    ```
* оповещения пользователей о забытых марафонах
    Для применения миграций следует вызвать функцию с телом:
    ```json
    {
      "magic_number": <MAGIC_NUMBER>,
      "notify_users": true
    }
    ```

    Например, так:
    ```shell
    curl -X POST https://functions.yandexcloud.net/<function-id> 
       -H "Content-Type: application/json"
       -d '{"magic_number":<MAGIC_NUMBER>,"notify_users":true}'  
    ```
* принудительной ротации статистики
    При этом:
    * Записи о марафонах за текущий день сбрасываются в ноль. 
    * Накопленное количество увеличивается на количество записей за текущий день
    * Если в текущем дне не было записей марафона - накопленное количество сбрасывается в ноль
 
    Для применения миграций следует вызвать функцию с телом:
    ```json
    {
      "magic_number": <MAGIC_NUMBER>,
      "rotate_stats": true
    }
    ```

  Например, так:
    ```shell
    curl -X POST https://functions.yandexcloud.net/<function-id> 
       -H "Content-Type: application/json"
       -d '{"magic_number":<MAGIC_NUMBER>,"rotate_stats":true}'  
    ```

