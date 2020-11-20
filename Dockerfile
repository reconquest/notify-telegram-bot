FROM alpine:edge

RUN apk update && apk add \
    bash \
    ca-certificates \
    && rm -rf /var/cache/apk/*

COPY notify-telegram-bot /bin/app
COPY config.toml /etc/bot.conf

CMD ["/bin/app", "--config=/etc/bot.conf"]
