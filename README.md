- [X] Простой UI с инпутом для ссылки на репозиторий и кнопкой для отправки запроса
- [X] Написать сервер, который примет запрос с UI и под капотом клонирует репозиторий во временную папку на сервере
- [X] Сбилдить проект с помощью Dockerfile в корне репозитория и запустить его, пробросив порт 80 контейнера на любой свободный порт сервера (надо уметь получать свободный порт) 
- [X] Удалил временный репозиторий после билда
- [ ] Сгенерировать новый поддомен для нового деплоя (например, `deploy-<timestamp>.<domain>`)
- [ ] Добавить новое правило в nginx (в будущем заменить на traefik) для проксирования запросов на новый контейнер с новым доменом
- [ ] Список деплоев на UI где можно увидеть статус и url
