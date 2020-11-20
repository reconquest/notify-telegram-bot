# Notify-telegram-bot

**Notify-telegram-bot** is a tool for sending notifications about updated fields in JSON 



## Installation

```
go get gitlab.com/reconquest/notify-telegram-bot
```

## Configuration

**Notify-telegram-bot** must be configured before using, the configuration file should be
placed in `config.toml` and must be written using the following syntax:

```toml
telegrambot_token = "token-example:203498455:BBFq-Mf5U64APalfdB2j3Z7XQfG5-MfVhett7"
uri_db = "example:mongodb://localhost:27017"
database_name = "example:Notify-telegram-bot"
main_collection = "endpoints"
subscribe_collection= "subscriptions"
```


## Requirements

### Step 1 Configure Telegram Bot Token

For setup telegram bot token, use telegram BotFather and follow the steps, provided BotFather


###  Step 2 Configure database

This program uses Mongo database.

1)URI is a necessary parameter to connect to the database.
* More information about URI you can find on the official Mongo documentation page - <https://docs.mongodb.com/manual/reference/connection-string/> 


* The standard URI connection scheme has the form:
    ```
    mongodb://[username:password@]host1[:port1][,...hostN[:portN]][/[defaultauthdb][?options]]
    ```

* By default Mongo database `URI="mongodb://localhost:27017"`.

2)For configuring `database_name` you can use any string name.




