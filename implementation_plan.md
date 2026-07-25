# OpenDeploy — Architecture v1.0

> Статус: **На утверждении**
> Версия: MVP 1.0
> Дата: 2026-07-25

---

## 1. Общая архитектура

OpenDeploy состоит из **трёх независимых процессов**, каждый из которых может работать и деплоиться отдельно:

```
┌─────────────────────────────────────────────────────────────────┐
│                        CLIENT LAYER                             │
│              Vue 3 + TypeScript + Vite + TailwindCSS            │
│                    (SPA, статические файлы)                     │
└────────────────────────────┬────────────────────────────────────┘
                             │ HTTPS / REST + WebSocket
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                       BACKEND LAYER                             │
│                    OpenDeploy Core (Go)                         │
│   HTTP API · Auth · Module Registry · WebSocket · Static FS     │
│                     Порт: 5888                                  │
└────────────────────────────┬────────────────────────────────────┘
                             │ gRPC (Unix Socket / TCP)
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                       AGENT LAYER                               │
│                  OpenDeploy Agent (Go)                          │
│   Shell · systemd · Package Manager · File System · Firewall    │
│              (только root, изолирован от сети)                  │
└─────────────────────────────────────────────────────────────────┘
```

### Почему три процесса, а не монолит?

| Аспект | Монолит | Три процесса (OpenDeploy) |
|---|---|---|
| Безопасность | Backend имеет root | Backend работает как обычный пользователь |
| Изоляция | Любой баг → root-shell | Баг в API ≠ root-доступ |
| Масштабируемость | Нельзя масштабировать части | Backend можно запускать удалённо |
| Обновление | Обновляем всё сразу | Agent и Core обновляются независимо |

---

## 2. Дерево каталогов

```
opendeploy/
│
├── cmd/                          # Точки входа (main пакеты)
│   ├── core/                     # Запуск OpenDeploy Core (HTTP API)
│   │   └── main.go
│   ├── agent/                    # Запуск OpenDeploy Agent (gRPC)
│   │   └── main.go
│   └── cli/                      # CLI утилита для управления
│       └── main.go
│
├── internal/                     # Внутренний код, не экспортируемый
│   │
│   ├── core/                     # Ядро системы
│   │   ├── app/                  # Application bootstrapper (DI, lifecycle)
│   │   │   ├── app.go            # App struct — корень всего DI-графа
│   │   │   └── config.go         # Загрузка и валидация конфига
│   │   │
│   │   ├── server/               # HTTP сервер
│   │   │   ├── server.go         # Настройка, lifecycle
│   │   │   ├── router.go         # Регистрация маршрутов
│   │   │   └── middleware/       # Middleware цепочка
│   │   │       ├── auth.go       # JWT валидация
│   │   │       ├── csrf.go       # CSRF защита
│   │   │       ├── ratelimit.go  # Rate limiting
│   │   │       ├── logger.go     # Request logging
│   │   │       ├── cors.go       # CORS политики
│   │   │       └── recover.go    # Panic recovery
│   │   │
│   │   ├── auth/                 # Домен авторизации
│   │   │   ├── domain.go         # User, Session, Role, Permission (entities)
│   │   │   ├── repository.go     # Интерфейс хранилища
│   │   │   ├── service.go        # Бизнес-логика (login, logout, refresh)
│   │   │   ├── handler.go        # HTTP handlers
│   │   │   └── jwt.go            # JWT утилиты
│   │   │
│   │   ├── module/               # Модульная система (реестр)
│   │   │   ├── registry.go       # ModuleRegistry — регистрация и поиск
│   │   │   ├── loader.go         # Автообнаружение и загрузка модулей
│   │   │   ├── lifecycle.go      # Enable/Disable/Install/Remove
│   │   │   ├── domain.go         # Module entity, ModuleState
│   │   │   ├── repository.go     # Интерфейс хранилища модулей
│   │   │   ├── service.go        # Бизнес-логика модулей
│   │   │   └── handler.go        # HTTP handlers для /api/modules
│   │   │
│   │   ├── site/                 # Домен сайтов
│   │   │   ├── domain.go         # Site entity
│   │   │   ├── repository.go     # Интерфейс хранилища
│   │   │   ├── service.go        # Бизнес-логика
│   │   │   └── handler.go        # HTTP handlers
│   │   │
│   │   ├── service/              # Домен systemd-служб
│   │   │   ├── domain.go         # SystemService entity
│   │   │   ├── service.go        # Бизнес-логика
│   │   │   └── handler.go        # HTTP handlers
│   │   │
│   │   ├── settings/             # Домен настроек системы
│   │   │   ├── domain.go         # Setting entity (key-value + typed)
│   │   │   ├── repository.go     # Интерфейс хранилища
│   │   │   ├── service.go        # Бизнес-логика
│   │   │   └── handler.go        # HTTP handlers
│   │   │
│   │   ├── dashboard/            # Домен дашборда
│   │   │   ├── service.go        # Агрегация метрик
│   │   │   └── handler.go        # HTTP handlers + WebSocket stream
│   │   │
│   │   └── audit/                # Аудит-лог
│   │       ├── domain.go         # AuditEntry entity
│   │       ├── repository.go     # Интерфейс хранилища
│   │       └── service.go        # Запись событий
│   │
│   ├── agent/                    # OpenDeploy Agent
│   │   ├── app/
│   │   │   └── app.go            # Agent bootstrapper
│   │   ├── server/               # gRPC сервер
│   │   │   └── server.go
│   │   ├── executor/             # Исполнитель команд
│   │   │   ├── shell.go          # Безопасное выполнение shell
│   │   │   ├── validator.go      # Валидация команд (allowlist)
│   │   │   └── sanitizer.go      # Санитизация аргументов
│   │   ├── systemd/              # systemd интеграция
│   │   │   └── manager.go        # Start/Stop/Enable/Disable/Logs
│   │   ├── packages/             # Управление пакетами
│   │   │   ├── manager.go        # Интерфейс PackageManager
│   │   │   ├── apt.go            # APT реализация
│   │   │   └── dnf.go            # DNF/YUM реализация
│   │   ├── filesystem/           # Работа с файлами
│   │   │   └── manager.go        # Read/Write/Delete/Chmod (с валидацией путей)
│   │   └── firewall/             # Управление firewall
│   │       ├── manager.go        # Интерфейс FirewallManager
│   │       └── ufw.go            # UFW реализация
│   │
│   ├── agentclient/              # gRPC клиент (используется Core для вызова Agent)
│   │   ├── client.go             # AgentClient struct
│   │   └── pool.go               # Connection pool
│   │
│   └── platform/                 # Общая инфраструктура
│       ├── database/             # Database layer
│       │   ├── database.go       # Интерфейс Database
│       │   ├── sqlite/           # SQLite реализация
│       │   │   └── sqlite.go
│       │   └── migrations/       # SQL миграции (embed)
│       │       ├── 001_init.sql
│       │       ├── 002_modules.sql
│       │       └── 003_sites.sql
│       ├── logger/               # Структурированное логирование
│       │   └── logger.go         # slog-обёртка с контекстом
│       ├── config/               # Конфигурация
│       │   ├── config.go         # Основной Config struct
│       │   └── loader.go         # YAML + env + defaults
│       ├── events/               # Event Bus (pub/sub внутри процесса)
│       │   ├── bus.go            # EventBus interface
│       │   └── memory.go         # In-memory реализация
│       ├── websocket/            # WebSocket hub
│       │   └── hub.go            # Broadcast, room-based messaging
│       └── errors/               # Типизированные ошибки
│           └── errors.go         # AppError, ErrorCode, HTTP mapping
│
├── modules/                      # Встроенные модули (каждый — независимый пакет)
│   ├── nginx/                    # Модуль Nginx
│   │   ├── module.go             # Реализация Module интерфейса
│   │   ├── service.go            # Nginx-специфичная логика
│   │   ├── handler.go            # HTTP endpoints модуля
│   │   ├── templates/            # Шаблоны конфигов Nginx
│   │   │   ├── site.conf.tmpl
│   │   │   └── php-fpm.conf.tmpl
│   │   └── tasks/                # Фоновые задачи модуля
│   │       └── healthcheck.go
│   │
│   ├── php/                      # Модуль PHP
│   │   ├── module.go
│   │   ├── service.go
│   │   ├── handler.go
│   │   └── versions.go           # Управление несколькими версиями PHP
│   │
│   ├── nodejs/                   # Модуль Node.js
│   │   ├── module.go
│   │   ├── service.go
│   │   └── handler.go
│   │
│   └── git/                      # Модуль Git
│       ├── module.go
│       ├── service.go
│       └── handler.go
│
├── pkg/                          # Публичные переиспользуемые пакеты
│   ├── contract/                 # Публичные интерфейсы (контракты)
│   │   ├── module.go             # Module interface — главный контракт
│   │   ├── agent.go              # AgentClient interface
│   │   └── event.go              # Event interface
│   └── version/                  # Версионирование
│       └── version.go
│
├── proto/                        # Protobuf определения (Core ↔ Agent)
│   └── agent/
│       └── v1/
│           ├── agent.proto       # Сервисы gRPC
│           └── agent.pb.go       # Сгенерированный код
│
├── web/                          # Frontend (Vue 3)
│   ├── src/
│   │   ├── main.ts
│   │   ├── App.vue
│   │   ├── router/               # Vue Router
│   │   │   └── index.ts
│   │   ├── stores/               # Pinia stores
│   │   │   ├── auth.ts
│   │   │   ├── modules.ts
│   │   │   ├── sites.ts
│   │   │   └── dashboard.ts
│   │   ├── api/                  # API клиент (типизированный)
│   │   │   ├── client.ts         # Axios instance + interceptors
│   │   │   ├── auth.ts
│   │   │   ├── modules.ts
│   │   │   ├── sites.ts
│   │   │   └── dashboard.ts
│   │   ├── views/                # Страницы
│   │   │   ├── LoginView.vue
│   │   │   ├── DashboardView.vue
│   │   │   ├── ModulesView.vue
│   │   │   ├── SitesView.vue
│   │   │   ├── ServicesView.vue
│   │   │   └── SettingsView.vue
│   │   ├── components/           # Компоненты
│   │   │   ├── layout/           # Shell, Sidebar, Header
│   │   │   ├── dashboard/        # Metric cards, charts
│   │   │   ├── modules/          # ModuleCard, ModuleList
│   │   │   ├── sites/            # SiteForm, SiteCard
│   │   │   ├── services/         # ServiceRow, LogViewer
│   │   │   └── ui/               # Button, Modal, Badge, Toast...
│   │   └── types/                # TypeScript типы (зеркало Go-моделей)
│   │       ├── module.ts
│   │       ├── site.ts
│   │       └── auth.ts
│   ├── index.html
│   ├── vite.config.ts
│   ├── tailwind.config.ts
│   └── package.json
│
├── configs/                      # Конфигурационные файлы по умолчанию
│   └── opendeploy.yaml           # Дефолтный конфиг
│
├── scripts/                      # Скрипты сборки и установки
│   ├── install.sh                # Установка на сервер
│   ├── build.sh                  # Сборка всего проекта
│   └── dev.sh                    # Запуск в режиме разработки
│
├── deployments/                  # Конфиги для деплоя
│   └── systemd/
│       ├── opendeploy-core.service
│       └── opendeploy-agent.service
│
├── docs/                         # Документация
│   ├── ARCHITECTURE.md
│   ├── API.md
│   └── adr/                      # Architecture Decision Records
│       ├── 001-grpc-agent.md
│       └── 002-module-system.md
│
├── go.mod
├── go.sum
├── Makefile
├── README.md
├── ARCHITECTURE.md
├── INSTALL.md
├── API.md
├── CHANGELOG.md
├── CONTRIBUTING.md
├── SECURITY.md
└── ROADMAP.md
```

---

## 3. Назначение ключевых директорий

| Директория | Назначение |
|---|---|
| `cmd/` | Точки входа. Каждый бинарник собирается из своей папки. `core`, `agent`, `cli` — разные процессы |
| `internal/core/` | Вся бизнес-логика панели. Недоступна снаружи модуля |
| `internal/agent/` | Вся логика системного агента. Работает под root |
| `internal/agentclient/` | gRPC-клиент для обращения Core → Agent |
| `internal/platform/` | Инфраструктурный слой: БД, логи, конфиг, события |
| `modules/` | Встроенные модули. Каждый — изолированный пакет, реализующий `Module` интерфейс |
| `pkg/contract/` | **Публичные интерфейсы**. Единственное, что `modules/` знают о `core/` |
| `proto/` | Контракт gRPC между Core и Agent |
| `web/` | Frontend-приложение. Собирается Vite и embed-ится в бинарник Core |

---

## 4. Схема взаимодействия компонентов

```
 Browser
    │
    │  HTTP/WSS :5888
    ▼
┌──────────────────────────────────────────┐
│           OpenDeploy Core                │
│                                          │
│  ┌─────────────┐   ┌──────────────────┐  │
│  │ HTTP Server │   │  WebSocket Hub   │  │
│  │  (chi/net)  │   │  (real-time)     │  │
│  └──────┬──────┘   └────────┬─────────┘  │
│         │                   │            │
│  ┌──────▼──────────────────▼──────────┐  │
│  │         Middleware Chain           │  │
│  │  Auth → CSRF → RateLimit → Logger  │  │
│  └──────────────────┬─────────────────┘  │
│                     │                   │
│  ┌──────────────────▼─────────────────┐  │
│  │           Domain Services          │  │
│  │  Auth │ Module │ Site │ Dashboard  │  │
│  └──────────────────┬─────────────────┘  │
│                     │                   │
│  ┌──────────────────▼─────────────────┐  │
│  │         Module Registry            │  │
│  │   nginx │ php │ nodejs │ git       │  │
│  └──────────────────┬─────────────────┘  │
│                     │                   │
│  ┌──────────────────▼─────────────────┐  │
│  │          AgentClient (gRPC)        │  │
│  └──────────────────┬─────────────────┘  │
└─────────────────────┼────────────────────┘
                      │ gRPC (Unix Socket)
                      ▼
┌──────────────────────────────────────────┐
│          OpenDeploy Agent                │
│                                          │
│  ┌───────────────────────────────────┐   │
│  │      Command Validator            │   │
│  │   (allowlist, sanitize args)      │   │
│  └──────────────┬────────────────────┘   │
│                 │                        │
│  ┌──────────────▼────────────────────┐   │
│  │          Executor Router          │   │
│  └──┬──────────┬──────────┬──────────┘   │
│     │          │          │              │
│  Shell    systemd    PackageManager      │
│  Executor  Manager   (apt/dnf)          │
│                                          │
│  ┌────────────────────────────────────┐  │
│  │  FileSystem Manager │ Firewall     │  │
│  └────────────────────────────────────┘  │
└──────────────────────────────────────────┘
```

---

## 5. Модель данных

### Таблицы SQLite

```sql
-- Пользователи
CREATE TABLE users (
    id          TEXT PRIMARY KEY,  -- UUID
    username    TEXT NOT NULL UNIQUE,
    email       TEXT NOT NULL UNIQUE,
    password    TEXT NOT NULL,     -- bcrypt hash
    role        TEXT NOT NULL DEFAULT 'operator',  -- admin | operator | viewer
    created_at  DATETIME NOT NULL,
    updated_at  DATETIME NOT NULL,
    last_login  DATETIME
);

-- Сессии (refresh tokens)
CREATE TABLE sessions (
    id          TEXT PRIMARY KEY,  -- UUID
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  TEXT NOT NULL UNIQUE,
    ip_address  TEXT NOT NULL,
    user_agent  TEXT,
    expires_at  DATETIME NOT NULL,
    created_at  DATETIME NOT NULL
);

-- Модули
CREATE TABLE modules (
    id          TEXT PRIMARY KEY,  -- Slug: "nginx", "php", "nodejs"
    name        TEXT NOT NULL,
    version     TEXT,              -- Установленная версия
    state       TEXT NOT NULL DEFAULT 'available',
                                   -- available | installing | installed
                                   -- | enabled | disabled | removing | error
    config      TEXT,              -- JSON blob модуль-специфичного конфига
    installed_at DATETIME,
    updated_at  DATETIME NOT NULL
);

-- Сайты
CREATE TABLE sites (
    id          TEXT PRIMARY KEY,  -- UUID
    domain      TEXT NOT NULL UNIQUE,
    root_path   TEXT NOT NULL,
    php_version TEXT,              -- NULL если не PHP сайт
    ssl_enabled INTEGER NOT NULL DEFAULT 0,
    state       TEXT NOT NULL DEFAULT 'active',  -- active | disabled | error
    created_at  DATETIME NOT NULL,
    updated_at  DATETIME NOT NULL
);

-- Настройки
CREATE TABLE settings (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL,
    updated_at  DATETIME NOT NULL
);

-- Аудит-лог
CREATE TABLE audit_log (
    id          TEXT PRIMARY KEY,  -- UUID
    user_id     TEXT REFERENCES users(id) ON DELETE SET NULL,
    action      TEXT NOT NULL,     -- "module.install", "site.create"
    resource    TEXT,              -- "nginx", "site:uuid"
    metadata    TEXT,              -- JSON blob с деталями
    ip_address  TEXT,
    status      TEXT NOT NULL,     -- "success" | "error"
    created_at  DATETIME NOT NULL
);

-- Фоновые задачи
CREATE TABLE jobs (
    id          TEXT PRIMARY KEY,  -- UUID
    type        TEXT NOT NULL,     -- "install_module", "create_site"
    payload     TEXT NOT NULL,     -- JSON
    state       TEXT NOT NULL DEFAULT 'pending',
                                   -- pending | running | success | error
    output      TEXT,              -- Накопленный вывод
    error       TEXT,
    created_at  DATETIME NOT NULL,
    started_at  DATETIME,
    finished_at DATETIME
);
```

### Индексы

```sql
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
CREATE INDEX idx_audit_log_user_id ON audit_log(user_id);
CREATE INDEX idx_audit_log_created_at ON audit_log(created_at);
CREATE INDEX idx_jobs_state ON jobs(state);
```

---

## 6. Ключевые интерфейсы (Go contracts)

### 6.1 Module Interface (главный контракт системы)

```go
// pkg/contract/module.go

// Module — контракт, который должен реализовать каждый модуль.
// Ядро знает только этот интерфейс.
type Module interface {
    // Метаданные
    ID() string          // уникальный slug: "nginx", "php"
    Name() string        // человекочитаемое название
    Version() string     // версия модуля (не пакета)
    Description() string

    // Жизненный цикл
    Bootstrap(deps ModuleDeps) error  // вызывается при старте, получает зависимости
    Shutdown(ctx context.Context) error

    // Регистрация возможностей (вызывается ядром при загрузке)
    RegisterRoutes(r Router)          // HTTP routes модуля
    RegisterMenuItems() []MenuItem    // Пункты меню
    RegisterSettings() []SettingSpec  // Настройки модуля

    // Управление (делегируется в Agent)
    Install(ctx context.Context) error
    Uninstall(ctx context.Context) error
    Enable(ctx context.Context) error
    Disable(ctx context.Context) error

    // Мониторинг
    Status(ctx context.Context) (*ModuleStatus, error)
    HealthCheck(ctx context.Context) (*HealthReport, error)
}

// ModuleDeps — зависимости, которые ядро передаёт модулю
type ModuleDeps struct {
    Agent   AgentClient   // для системных операций
    DB      Database      // для хранения данных
    Events  EventBus      // для публикации событий
    Logger  Logger        // структурированный логгер
    Config  ModuleConfig  // конфиг модуля
}
```

### 6.2 AgentClient Interface

```go
// pkg/contract/agent.go

type AgentClient interface {
    // Системные службы
    ServiceStart(ctx context.Context, name string) error
    ServiceStop(ctx context.Context, name string) error
    ServiceRestart(ctx context.Context, name string) error
    ServiceEnable(ctx context.Context, name string) error
    ServiceDisable(ctx context.Context, name string) error
    ServiceStatus(ctx context.Context, name string) (*ServiceStatus, error)
    ServiceLogs(ctx context.Context, name string, lines int) ([]string, error)

    // Пакеты
    PackageInstall(ctx context.Context, pkg string) (<-chan string, error) // stream output
    PackageRemove(ctx context.Context, pkg string) (<-chan string, error)
    PackageUpdate(ctx context.Context, pkg string) (<-chan string, error)
    PackageInstalled(ctx context.Context, pkg string) (bool, string, error)

    // Файловая система
    FileRead(ctx context.Context, path string) ([]byte, error)
    FileWrite(ctx context.Context, path string, content []byte, mode os.FileMode) error
    FileDelete(ctx context.Context, path string) error
    DirCreate(ctx context.Context, path string, mode os.FileMode) error
    DirList(ctx context.Context, path string) ([]FileInfo, error)

    // Firewall
    FirewallAllow(ctx context.Context, port int, proto string) error
    FirewallDeny(ctx context.Context, port int, proto string) error
    FirewallList(ctx context.Context) ([]FirewallRule, error)

    // Системная информация
    SystemStats(ctx context.Context) (*SystemStats, error)
}
```

### 6.3 Repository Interface (пример для User)

```go
// internal/core/auth/repository.go

type UserRepository interface {
    FindByID(ctx context.Context, id string) (*User, error)
    FindByUsername(ctx context.Context, username string) (*User, error)
    FindByEmail(ctx context.Context, email string) (*User, error)
    Create(ctx context.Context, user *User) error
    Update(ctx context.Context, user *User) error
    Delete(ctx context.Context, id string) error
}

type SessionRepository interface {
    Create(ctx context.Context, session *Session) error
    FindByTokenHash(ctx context.Context, hash string) (*Session, error)
    DeleteByID(ctx context.Context, id string) error
    DeleteExpired(ctx context.Context) error
    DeleteByUserID(ctx context.Context, userID string) error
}
```

### 6.4 EventBus Interface

```go
// pkg/contract/event.go

type Event interface {
    Type() string
    Payload() any
    OccurredAt() time.Time
}

type EventBus interface {
    Publish(ctx context.Context, event Event) error
    Subscribe(eventType string, handler EventHandler) (UnsubscribeFn, error)
}
```

---

## 7. Структура модульной системы

```
Lifecycle при старте Core:
─────────────────────────

1. App.Bootstrap()
        │
        ▼
2. ModuleLoader.Discover()
   └─ Сканирует все зарегистрированные модули (compile-time регистрация)
   └─ Создаёт экземпляры (Module interface)
        │
        ▼
3. Для каждого модуля:
   a. Читает состояние из БД (installed? enabled?)
   b. Если enabled → вызывает Module.Bootstrap(deps)
   c. Module.RegisterRoutes(router) — добавляет свои /api/modules/{id}/...
   d. Module.RegisterMenuItems() — добавляет пункты в меню
        │
        ▼
4. Ядро готово к приёму запросов

Lifecycle Module.Install():
───────────────────────────

HTTP POST /api/modules/{id}/install
        │
        ▼
ModuleService.Install(ctx, id)
        │
        ├─ Создаёт Job в БД (state=pending)
        │
        ├─ Возвращает jobID клиенту (202 Accepted)
        │
        └─ async goroutine:
               │
               ├─ Job state = running
               │
               ├─ Module.Install(ctx)
               │   └─ вызывает AgentClient.PackageInstall(...)
               │       └─ gRPC → Agent → apt install -y nginx
               │           └─ stream output → Job.output (через EventBus → WebSocket)
               │
               ├─ Module state = installed → БД
               │
               └─ Job state = success
```

---

## 8. Структура System Agent

### Слои агента

```
gRPC Request
     │
     ▼
┌──────────────────────────────────────┐
│         AgentServer (gRPC)           │  ← Только приём запросов, нет логики
└──────────────────────┬───────────────┘
                       │
                       ▼
┌──────────────────────────────────────┐
│       CommandValidator               │  ← Allowlist команд, санитизация
│                                      │    аргументов, проверка путей
└──────────────────────┬───────────────┘
                       │
                       ▼
┌──────────────────────────────────────┐
│          Executor Router             │  ← Маршрутизация по типу запроса
└──┬──────────┬──────────┬─────────────┘
   │          │          │
   ▼          ▼          ▼
Shell      systemd   PackageManager
Executor   Manager   (apt/dnf/yum)
   │
   ▼
Process with:
  - Ограниченный PATH
  - Timeout
  - Output capture
  - Audit log запись
```

### Безопасность Agent

```
1. gRPC соединение только по Unix Socket (по умолчанию)
   → Core и Agent работают на одном хосте
   → Нет сетевого доступа к Agent снаружи

2. Взаимная аутентификация
   → mTLS или shared secret в заголовке gRPC

3. Command Allowlist
   → Agent НЕ выполняет произвольные команды
   → Только предопределённый список операций (proto enum)

4. Path Validation
   → Все пути проверяются на path traversal
   → Запрещены символические ссылки вне разрешённых директорий

5. Audit Log
   → Каждая операция Agent пишет в audit_log
```

---

## 9. Структура API

### Маршруты ядра

| Метод | Путь | Описание |
|---|---|---|
| POST | `/api/v1/auth/login` | Вход, получение JWT |
| POST | `/api/v1/auth/logout` | Выход, инвалидация токена |
| POST | `/api/v1/auth/refresh` | Обновление access token |
| GET | `/api/v1/auth/me` | Текущий пользователь |
| GET | `/api/v1/dashboard/stats` | Системные метрики |
| GET | `/api/v1/dashboard/stream` | WebSocket: live метрики |
| GET | `/api/v1/modules` | Список всех модулей |
| GET | `/api/v1/modules/:id` | Детали модуля |
| POST | `/api/v1/modules/:id/install` | Установить модуль → Job |
| POST | `/api/v1/modules/:id/uninstall` | Удалить модуль → Job |
| POST | `/api/v1/modules/:id/enable` | Включить |
| POST | `/api/v1/modules/:id/disable` | Отключить |
| POST | `/api/v1/modules/:id/restart` | Перезапустить |
| GET | `/api/v1/modules/:id/status` | Статус и health |
| GET | `/api/v1/modules/:id/settings` | Настройки модуля |
| PUT | `/api/v1/modules/:id/settings` | Обновить настройки |
| GET | `/api/v1/sites` | Список сайтов |
| POST | `/api/v1/sites` | Создать сайт |
| GET | `/api/v1/sites/:id` | Детали сайта |
| PUT | `/api/v1/sites/:id` | Обновить сайт |
| DELETE | `/api/v1/sites/:id` | Удалить сайт |
| POST | `/api/v1/sites/:id/enable` | Включить сайт |
| POST | `/api/v1/sites/:id/disable` | Отключить сайт |
| GET | `/api/v1/services` | Список служб |
| POST | `/api/v1/services/:name/start` | Запустить |
| POST | `/api/v1/services/:name/stop` | Остановить |
| POST | `/api/v1/services/:name/restart` | Перезапустить |
| GET | `/api/v1/services/:name/logs` | Логи службы |
| GET | `/api/v1/settings` | Все настройки |
| PUT | `/api/v1/settings` | Обновить настройки |
| GET | `/api/v1/jobs/:id` | Статус фоновой задачи |
| GET | `/api/v1/audit` | Аудит-лог |
| GET | `/health` | Health check (без авторизации) |

### Маршруты модулей (регистрируются самими модулями)

| Метод | Путь | Модуль |
|---|---|---|
| GET | `/api/v1/modules/php/versions` | PHP — список версий |
| POST | `/api/v1/modules/php/versions/:v/install` | PHP — установить версию |
| GET | `/api/v1/modules/nginx/vhosts` | Nginx — виртуальные хосты |
| POST | `/api/v1/modules/git/deploy/:site` | Git — деплой сайта |

---

## 10. gRPC контракт (Agent)

```protobuf
// proto/agent/v1/agent.proto

service AgentService {
    // Службы
    rpc ServiceAction (ServiceActionRequest) returns (ServiceActionResponse);
    rpc ServiceStatus (ServiceStatusRequest) returns (ServiceStatusResponse);
    rpc ServiceLogs   (ServiceLogsRequest)   returns (stream LogLine);

    // Пакеты
    rpc PackageInstall (PackageRequest) returns (stream PackageOutput);
    rpc PackageRemove  (PackageRequest) returns (stream PackageOutput);
    rpc PackageStatus  (PackageRequest) returns (PackageStatusResponse);

    // Файлы
    rpc FileRead   (FileReadRequest)   returns (FileReadResponse);
    rpc FileWrite  (FileWriteRequest)  returns (FileWriteResponse);
    rpc FileDelete (FileDeleteRequest) returns (FileDeleteResponse);
    rpc DirCreate  (DirCreateRequest)  returns (DirCreateResponse);
    rpc DirList    (DirListRequest)    returns (DirListResponse);

    // Firewall
    rpc FirewallRule (FirewallRuleRequest) returns (FirewallRuleResponse);
    rpc FirewallList (FirewallListRequest) returns (FirewallListResponse);

    // Система
    rpc SystemStats (SystemStatsRequest) returns (SystemStatsResponse);
}
```

---

## 11. Поток выполнения запроса

### Пример: Установка модуля Nginx

```
[Browser] POST /api/v1/modules/nginx/install
     │
     ▼ [Middleware: Auth]
     JWT валидация, проверка роли (admin only)
     │
     ▼ [Middleware: CSRF]
     Проверка CSRF токена
     │
     ▼ [Middleware: RateLimit]
     Проверка лимитов запросов
     │
     ▼ [ModuleHandler.Install]
     Валидация запроса
     │
     ▼ [ModuleService.Install(ctx, "nginx")]
     Проверка: модуль существует? уже установлен?
     │
     ├─ Запись в audit_log: action="module.install", status="pending"
     │
     ├─ Создание Job {id: uuid, type: "install_module", state: "pending"}
     │
     ├─ HTTP 202 Accepted → {job_id: "..."}  ← Ответ клиенту
     │
     └─ [Async goroutine]
           │
           ▼ [AgentClient.PackageInstall(ctx, "nginx")]
           gRPC → Agent
           │
           ▼ [Agent: CommandValidator]
           Разрешена ли операция "package_install" с пакетом "nginx"?
           │
           ▼ [Agent: PackageManager.Install("nginx")]
           exec: apt-get install -y nginx
           │
           └─ Стриминг stdout → gRPC stream → Job.output
               → EventBus.Publish(JobOutputEvent)
               → WebSocket Hub → Browser (live output)
           │
           ▼ Завершение
           Job.state = "success"
           Module.state = "installed"
           Запись в audit_log: status="success"
           EventBus.Publish(ModuleInstalledEvent)

[Browser] WebSocket получает события в реальном времени
```

---

## 12. Жизненный цикл приложения

```
systemd start opendeploy-agent.service
    └─ Agent инициализирует gRPC сервер на Unix Socket
    └─ Готов принимать запросы

systemd start opendeploy-core.service
    └─ 1. Загрузка конфига (YAML + env overrides)
    └─ 2. Инициализация Logger
    └─ 3. Подключение к SQLite
    └─ 4. Запуск миграций БД
    └─ 5. Подключение к Agent (gRPC)
    └─ 6. Инициализация EventBus
    └─ 7. Инициализация WebSocket Hub
    └─ 8. Регистрация модулей (ModuleLoader.Discover)
          └─ Для каждого enabled модуля → Module.Bootstrap()
    └─ 9. Сборка DI-графа (Service → Handler → Router)
    └─ 10. Запуск HTTP сервера на :5888
    └─ 11. Запуск фоновых задач (сборщик метрик, планировщик задач)
    └─ READY

Graceful Shutdown (SIGTERM):
    └─ HTTP сервер: перестаёт принимать новые соединения
    └─ Ожидание завершения активных запросов (timeout: 30s)
    └─ Для каждого модуля → Module.Shutdown()
    └─ Закрытие соединения с Agent
    └─ Закрытие БД
    └─ EXIT 0
```

---

## 13. Конфигурация

```yaml
# configs/opendeploy.yaml

server:
  host: "0.0.0.0"
  port: 5888
  read_timeout: 30s
  write_timeout: 60s

database:
  driver: "sqlite"
  dsn: "/var/lib/opendeploy/data.db"

agent:
  socket: "/run/opendeploy-agent/agent.sock"
  timeout: 120s

auth:
  jwt_secret: ""            # Генерируется при первом старте
  access_token_ttl: 15m
  refresh_token_ttl: 7d

security:
  rate_limit:
    enabled: true
    requests_per_minute: 60
  csrf:
    enabled: true

logging:
  level: "info"             # debug | info | warn | error
  format: "json"            # json | text
  file: "/var/log/opendeploy/core.log"

modules:
  enabled:
    - nginx
    - php
    - nodejs
    - git
```

---

## 14. RBAC модель

```
Роли:
  admin    — полный доступ
  operator — управление сайтами и модулями, нет доступа к настройкам безопасности
  viewer   — только чтение

Разрешения:
  module:view      module:install   module:uninstall
  module:enable    module:disable   module:configure
  site:view        site:create      site:update     site:delete
  service:view     service:manage
  settings:view    settings:update  settings:security
  audit:view
  user:manage
```

---

## 15. План реализации MVP по этапам

### Этап 1 — Инфраструктурный фундамент (Core + Agent skeleton)
**Цель:** Работающий gRPC канал Core→Agent, HTTP сервер с auth

- [ ] Инициализация Go модуля, структура директорий
- [ ] `internal/platform/config` — загрузка конфига
- [ ] `internal/platform/logger` — slog с контекстом
- [ ] `internal/platform/database/sqlite` — подключение + миграции
- [ ] `internal/platform/errors` — типизированные ошибки
- [ ] `proto/agent/v1` — protobuf определения
- [ ] `internal/agent/` — gRPC сервер (stub handlers)
- [ ] `internal/agentclient/` — gRPC клиент
- [ ] `internal/core/server/` — HTTP сервер + middleware цепочка
- [ ] `cmd/core/main.go`, `cmd/agent/main.go`

**Проверка:** `go build ./...` без ошибок, Core и Agent стартуют

---

### Этап 2 — Авторизация и безопасность
**Цель:** Полноценная auth система

- [ ] `internal/core/auth/domain.go` — User, Session, Role
- [ ] `internal/core/auth/repository.go` — интерфейс
- [ ] SQLite реализация UserRepository и SessionRepository
- [ ] `internal/core/auth/service.go` — login, logout, refresh
- [ ] JWT (access + refresh токены)
- [ ] `middleware/auth.go` — JWT валидация
- [ ] `middleware/csrf.go`, `middleware/ratelimit.go`
- [ ] `internal/core/auth/handler.go` — HTTP endpoints
- [ ] `internal/core/audit/` — запись в audit_log

**Проверка:** POST /api/v1/auth/login возвращает токены

---

### Этап 3 — Модульная система и реестр
**Цель:** Работающий ModuleRegistry, регистрация роутов

- [ ] `pkg/contract/module.go` — Module interface
- [ ] `internal/core/module/registry.go`
- [ ] `internal/core/module/loader.go`
- [ ] `internal/core/module/lifecycle.go` + Jobs (async)
- [ ] `internal/platform/events/` — EventBus
- [ ] `internal/platform/websocket/` — WebSocket Hub
- [ ] `internal/core/module/handler.go` — CRUD + actions

**Проверка:** GET /api/v1/modules возвращает список, Job создаётся

---

### Этап 4 — System Agent (полная реализация)
**Цель:** Agent выполняет системные операции через gRPC

- [ ] `internal/agent/executor/validator.go` — allowlist
- [ ] `internal/agent/executor/shell.go` — безопасный exec
- [ ] `internal/agent/systemd/manager.go`
- [ ] `internal/agent/packages/apt.go`, `dnf.go`
- [ ] `internal/agent/filesystem/manager.go`
- [ ] `internal/agent/firewall/ufw.go`
- [ ] `internal/agent/server/server.go` — полная реализация proto

**Проверка:** Установка `curl` через API → apt install в системе

---

### Этап 5 — Dashboard и метрики
**Цель:** Реал-тайм метрики через WebSocket

- [ ] AgentClient.SystemStats() — CPU, RAM, Disk, Network
- [ ] `internal/core/dashboard/service.go` — агрегация
- [ ] `internal/core/dashboard/handler.go` — REST + WebSocket
- [ ] Горутина сбора метрик каждые 2 секунды

**Проверка:** GET /api/v1/dashboard/stats возвращает метрики

---

### Этап 6 — Сайты и службы
**Цель:** CRUD сайтов, управление systemd службами

- [ ] `internal/core/site/` — полная реализация
- [ ] `internal/core/service/` — полная реализация
- [ ] `modules/nginx/` — генерация vhost конфига

**Проверка:** Создание сайта → Nginx vhost создан и активирован

---

### Этап 7 — Первые модули
**Цель:** Рабочие модули Nginx, PHP, Node.js, Git

- [ ] `modules/nginx/` — Install, Status, HealthCheck
- [ ] `modules/php/` — Множество версий
- [ ] `modules/nodejs/` — Install, nvm-совместимость
- [ ] `modules/git/` — Deploy hooks

**Проверка:** Полный цикл: установка PHP 8.3 через UI

---

### Этап 8 — Frontend
**Цель:** Полноценный Vue 3 SPA

- [ ] Инициализация Vite + Vue 3 + TailwindCSS
- [ ] Design system (цвета, типографика, компоненты)
- [ ] Auth flow (Login, JWT хранение, interceptors)
- [ ] Dashboard с живыми метриками
- [ ] Модули UI (карточки, установка с live output)
- [ ] Сайты UI (CRUD форма)
- [ ] Службы UI
- [ ] Настройки UI
- [ ] Embed в бинарник Core (go embed)

---

### Этап 9 — Документация и деплой
**Цель:** Готовность к публикации

- [ ] README.md, INSTALL.md, ARCHITECTURE.md
- [ ] systemd service файлы
- [ ] install.sh скрипт
- [ ] Makefile (build, test, lint, dev)
- [ ] CHANGELOG.md, SECURITY.md, ROADMAP.md

---

## Открытые вопросы для обсуждения

> [!IMPORTANT]
> **Q1: Коммуникация Core ↔ Agent**
> Предлагается: **Unix Socket** по умолчанию (Core и Agent на одном хосте).
> Альтернатива: TCP с mTLS (если нужен удалённый Agent в будущем).
> Архитектурно лучше сразу заложить mTLS, даже для Unix Socket это не лишний overhead.

> [!IMPORTANT]
> **Q2: Регистрация модулей**
> Предлагается: **compile-time регистрация** (модули импортируются в `cmd/core/main.go` через `_` blank import).
> Альтернатива: Plugin system (`.so` файлы) — сложнее, нестабильнее в Go.
> Для MVP и open-source: compile-time, плагины — в roadmap.

> [!IMPORTANT]
> **Q3: HTTP роутер**
> Предлагается: **`net/http` + `chi`** (минимальная зависимость, идиоматичный Go).
> Альтернатива: Gin, Echo, Fiber.
> Chi — наиболее совместим со стандартной библиотекой, легко тестируется.

> [!NOTE]
> **Q4: Frontend билд**
> Vue SPA билдится Vite и embed-ится в бинарник Core через `go:embed`.
> Одна точка деплоя — только бинарник Core. Нет нужды в отдельном веб-сервере.
