# Armada

Агент управления узлом ARM-кластера. Pre-MVP: один Go-бинарник отдаёт веб-интерфейс, показывает метрики **этого** хоста и открывает интерактивную консоль в браузере.

Целевая платформа — **Luckfox Pico Max** (Alpine Linux, ARMv7). Для разработки можно запускать нативно или в Docker на машине разработчика.

## Возможности

- **Обзор** — CPU, RAM, диск `/`, температура (если есть в `/sys`), сеть, load average, график CPU
- **GPIO** — управление контактами Luckfox Pico Max (вход/выход, PWM), сохранение в SQLite
- **Настройки** — название системы, автообновление данных в UI
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
make armv7
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
| `-db-path` | Каталог SQLite-баз (по умолчанию `/etc/armada`): `settings.db`, `gpio.db` |
| `-no-gpio` | Заглушка GPIO (для Docker/x86) |
| `-iomux-dev` | Устройство Rockchip pinmux (по умолчанию `/dev/iomux`; пустая строка — не трогать mux) |
| `ARMADA_PASSWORD` | Если задан — Basic Auth на все маршруты, включая WebSocket |

`make run` использует `./data`; Docker Compose монтирует `./docker-data` в `/etc/armada` (каталог создаётся при `make up`).

Если `gpio.db` отсутствует или пуст, сохранённая конфигурация GPIO не применяется при старте.

### GPIO на Alpine (без luckfox-config)

На официальном Buildroot-образе Luckfox pinmux меняют через `luckfox-config`; в **Alpine его нет**. Варианты:

1. **Проще всего** — использовать контакты **GPIO1** (например 12, 14, 15, 16): они обычно уже в режиме GPIO.
2. **Без пересборки DTB** — на RV1106 armada сама переводит pad в GPIO: сначала `/dev/iomux`, если его нет — запись регистра IOMUX в IOC (`0xff538000`) через `/dev/mem`. Процесс должен быть **root**. Это не требует `luckfox-config` и не требует своего ядра у пользователя.
3. Линии, которые ядро уже заняло (`gpioinfo` показывает `consumer=`, например **SPI0 CS0** на контакте 12), так не освободить: драйвер SPI держит GPIO. Для них либо другой контакт, либо `status = "disabled"` у `&spi0`.

Контакты **1–2** заняты консолью UART2 и в UI не показываются. На контактах **UART3/UART4** в интерфейсе есть предупреждение.

**`device or resource busy`** — линию уже держит драйвер ядра. В логе armada будет `consumer=…` (например **`spi0 CS0`** на **контакте 12** — нужно `status = "disabled"` у `&spi0` в device tree, если SPI не используете). Без `gpioinfo`: `apk add libgpiod` (репозиторий **community** в `/etc/apk/repositories`) или смотрите consumer в логе armada. «Свободные» контакты для GPIO: **11**, **17**, **29**, **34**.

## API

```http
GET /api/v1/host
GET /api/v1/settings
PUT /api/v1/settings
POST /api/v1/gpio/db/reset
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
