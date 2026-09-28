# Armada

Агент управления узлом ARM-кластера. Pre-MVP: один Go-бинарник отдаёт веб-интерфейс, показывает метрики **этого** хоста и открывает интерактивную консоль в браузере.

Целевая платформа — **Luckfox Pico Max** (Alpine Linux, ARMv7). Для разработки можно запускать нативно или в Docker на машине разработчика.

## Возможности

- **Обзор** — CPU, RAM, диск `/`, температура (если есть в `/sys`), сеть, load average, график CPU
- **Консоль** — shell узла (`/bin/sh`) через WebSocket и xterm.js
- **API** — `GET /api/v1/host` (JSON-снимок для будущего контроллера кластера)
- **Опциональная авторизация** — HTTP Basic (`admin` + пароль из `ARMADA_PASSWORD`)

## Требования

- Go 1.22+ (для сборки)
- Linux с `/dev/ptmx` (консоль)
- Docker / Compose — только для локального Alpine-окружения

## Быстрый старт

```bash
# нативно (по умолчанию :18080 в Makefile)
make run

# другой порт
make run LISTEN=:8080
```

Откройте в браузере: [http://127.0.0.1:18080/#/](http://127.0.0.1:18080/#/)

Консоль: [http://127.0.0.1:18080/#/console](http://127.0.0.1:18080/#/console)

### Docker

```bash
make up
# UI на http://127.0.0.1:18080
```

Пароль (если нужен):

```bash
ARMADA_PASSWORD=secret make up
```

### Сборка для платы (ARMv7)

```bash
make build-armv7
# bin/armada-armv7
```

Скопируйте бинарник на устройство и запустите, например:

```bash
./armada-armv7 -listen :8080
```

### OpenRC (Alpine на плате)

Пример unit: [`deploy/armada.openrc`](deploy/armada.openrc).

## Конфигурация

| Параметр / переменная | Описание |
|----------------------|----------|
| `-listen` | Адрес HTTP-сервера (по умолчанию `:8080`) |
| `ARMADA_PASSWORD` | Если задан — Basic Auth на все маршруты, включая WebSocket |

## API

```http
GET /api/v1/host
```

Пример ответа (сокращённо):

```json
{
  "hostname": "luckfox",
  "cpu": { "usagePercent": 12.5, "load1": 0.4 },
  "ram": { "totalBytes": 268435456, "usagePercent": 42.1 },
  "disk": { "path": "/", "usagePercent": 38.0 },
  "network": [ { "name": "eth0", "rxBps": 1024, "txBps": 512 } ]
}
```

WebSocket консоли: `GET /ws/console` (бинарные кадры — вывод/ввод терминала; текстовый JSON `{"type":"resize","cols":80,"rows":24}`).

## Структура проекта

```
cmd/armada/          точка входа
internal/host/       сбор метрик из /proc и /sys
internal/httpapi/    HTTP, статика, WebSocket
internal/term/       PTY (Linux, без CGO)
web/                 UI (на основе assets/example.html)
assets/example.html  исходный макет интерфейса
deploy/              OpenRC
```

## Разработка

```bash
make test
```

После изменения файлов в `web/` перезапустите бинарник — статика вшита через `embed`.

## Дальше

- регистрация узлов в кластере и центральный контроллер
- управление несколькими нодами из одного UI
- TLS и полноценная аутентификация
