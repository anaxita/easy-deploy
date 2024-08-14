package entity

import (
	"github.com/gofrs/uuid/v5"
)

type Project struct {
	ID                uuid.UUID `json:"id"`                  // Генерируем автоматически
	URL               string    `json:"url"`                 // Ссылка на репозиторий
	Name              string    `json:"name"`                // Берём из ссылки на репозиторий
	Branch            string    `json:"branch"`              // Ветка репозитория которую деплоим
	DockerContainerID string    `json:"docker_container_id"` // ID запущенного контейнера
	Domain            string    `json:"domain"`              // Случайный домен для доступа к контейнеру
	AccessToken       string    `json:"access_token"`        // Токен доступа к репозиторию, если репозиторий приватный
}
