# OpenDeploy — Task Tracker

Актуализировано: 2026-07-25. Источник истины — фактическое состояние кода и
проверки сборки; подробные архитектурные решения находятся в
`implementation_plan.md`.

## Этап 1 — Инфраструктурный фундамент ✅

- [x] Go-модуль, `go.mod`, Makefile
- [x] Конфигурация, логирование, SQLite, типизированные ошибки
- [x] Event bus и WebSocket hub
- [x] Публичные контракты и protobuf-контракт Agent
- [x] Конфигурация и systemd units

## Этап 2 — Авторизация ✅

- [x] Auth domain, SQLite repositories, JWT, service и handlers
- [x] Audit log и auth middleware
- [x] Rate limiting

## Этап 3 — Модульная система ✅

- [x] Module contract, registry и loader
- [x] Repository, lifecycle service и HTTP handlers
- [x] Регистрация маршрутов в Core

## Этап 4 — System Agent ✅

- [x] Allowlist executor
- [x] systemd, APT/DNF, filesystem и UFW adapters
- [x] gRPC Agent server и Core client
- [x] CLI и точки входа Core/Agent

## Этап 5 — Dashboard и метрики ✅

- [x] Linux collector: CPU, RAM, Swap, Disk, Network, Load Average,
  температура и uptime
- [x] gRPC `SystemStats` и адаптер AgentClient
- [x] Dashboard aggregation service
- [x] REST snapshot API и WebSocket push
- [x] Периодическое сохранение и очистка metric snapshots
- [x] Миграция `004_dashboard.sql`

Примечание: таблицы `sessions` и `jobs` уже создаются миграцией
`001_init.sql`; отдельные повторные миграции для них не нужны.

## Этап 6 — Сайты, службы и настройки 🟡

- [x] Sites: domain, SQLite repository, service и HTTP handlers
- [x] Managed services: repository, service и HTTP handlers
- [x] Typed settings service и HTTP handlers
- [x] Миграция `005_services.sql`
- [x] Строгая валидация domain, document root и PHP version
- [x] Атомарная запись конфигурационных файлов на стороне Agent
- [x] Интеграция Sites с генерацией и атомарным применением Nginx vhost
- [x] `nginx -t`, reload и автоматический rollback через типизированный Agent RPC
- [ ] Полный поток подтверждения критических операций
- [/] Доменные и интеграционные тесты

## Этап 7 — Первые модули 🟡

- [x] Базовые модули Nginx, PHP, Node.js и Git
- [ ] Версионированная установка PHP
- [ ] Безопасная установка Node.js без пользовательских shell-скриптов
- [ ] Git deployment workflow
- [ ] Тесты каждого модуля

## Этап 8 — Frontend 🟡

- [x] Vue 3, Vite, Pinia, Vue Router и TailwindCSS
- [x] Auth flow и базовый application shell
- [x] Dashboard, Modules, Sites, Services и Settings views
- [ ] Миграция frontend-кода с JavaScript на TypeScript
- [ ] Полная обработка loading/error/empty states
- [ ] Подтверждение критических действий
- [ ] Frontend tests
- [x] Embed production build в бинарник Core

## Этап 9 — Документация и поставка 🟡

- [x] `README.md`, `ARCHITECTURE.md`, `API.md`, `CHANGELOG.md`
- [x] systemd units
- [ ] `SECURITY.md`, `ROADMAP.md`, `INSTALL.md`, `CONTRIBUTING.md`
- [ ] Безопасный install/uninstall workflow
- [ ] Release packaging и CI

## Текущий следующий шаг

Завершить этап 6: добавить единый механизм подтверждения критических операций
в API и frontend, затем расширить интеграционные тесты Site lifecycle.
