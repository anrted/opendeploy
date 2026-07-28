# OpenDeploy: полный технический аудит

Дата аудита: 2026-07-29
Аудируемая ревизия: `abac59152d027f041a5c1bd81bf91b6f36af5b39` (`main`)
Версия последнего опубликованного релиза в истории репозитория: `0.1.16`
Область проверки: Backend, Frontend, UI, Agent, gRPC, REST API, Installer,
Updater, CI/CD, тесты, документация и десять встроенных модулей.

## 1. Резюме

OpenDeploy вырос из набора ранних заготовок в работающий односерверный control
plane для Linux. В текущем состоянии проект умеет устанавливаться на чистую
Ubuntu-систему, разделяет непривилегированный Core и root Agent, управляет
сайтами, Nginx, systemd-службами, процессами, UFW, Fail2Ban, файлами,
пользователями и асинхронными задачами. Критические операции Nginx и пресетов
Fail2Ban валидируются и имеют компенсацию/rollback. Основной CI и полный Ubuntu
install/provision smoke проходят на ревизии аудита.

Проект при этом ещё нельзя считать production-ready панелью общего назначения.
Слабейшие места находятся не в базовом CRUD, а на эксплуатационных границах:
цепочка обновления не привязана криптографически к выбранному релизу, нет
штатного backup/restore, SPA хранит JWT в `localStorage`, JWT для streaming
логов службы передаётся через query string, gRPC upload объявлен, но не
реализован, frontend почти не покрыт тестами, а глубина встроенных модулей
неоднородна.

Текущая стадия: **late alpha / operational alpha**.

- Инженерная зрелость: **7.3/10**.
- Функциональная готовность продукта: **74%**.
- Production readiness: **61%**.
- Готовность к beta: **68%**.

Разница между функциональной и production-готовностью существенна намеренно:
root-панель требует доказанной безопасности обновлений, восстановления,
привилегированных операций и отказоустойчивости, а не только работающего UI.

## 2. Методика оценки

Проценты рассчитаны заново по текущему коду. Для каждого компонента учитывались:

1. полнота пользовательских сценариев — 30%;
2. безопасность и изоляция — 25%;
3. отказоустойчивость и rollback — 20%;
4. тестовое и CI-подтверждение — 15%;
5. документация и эксплуатационная поддержка — 10%.

Оценка 100% означает не «есть код», а завершённый, документированный и
проверенный production-сценарий. Заготовки интерфейсов и metadata без
действующего backend не засчитывались как готовая функция.

## 3. Проверенная база

| Показатель | Текущее состояние |
|---|---:|
| Go-файлы без сгенерированного protobuf | 132 |
| Строк Go-кода с generated bindings | около 24 500 |
| Frontend-код Vue/JavaScript | около 6 200 строк |
| Go test-файлы | 25 |
| Go-тесты | около 70 |
| Frontend test-файлы | 1 |
| Frontend-тесты | 3 |
| REST-маршруты в Core | около 76 |
| gRPC-методы Agent | 34 |
| Миграции БД | 8 |
| Встроенные модули | 10 |
| Основные SPA-маршруты | 10 |

Проверки на ревизии аудита:

- GitHub CI: backend `go test -race ./...`, golangci-lint, frontend tests,
  ESLint, govulncheck и npm audit job — основной workflow завершён успешно;
- Ubuntu smoke: сборка, установка, запуск Core/Agent, вход, включение пяти
  Fail2Ban-пресетов, создание Nginx-сайта, `nginx -t` и restart-equivalent
  проверка — успешно;
- frontend production build — успешно;
- локальные тесты изменённых Nginx/Fail2Ban/Core/Agent-пакетов — успешно.

На Windows полный локальный `go test ./...` не является репрезентативным:
`go-sqlite3` требует CGO/GCC, а текущая среда собрана с `CGO_ENABLED=0`.
Авторитетный Linux race-suite прошёл в CI.

## 4. Готовность компонентов

| Компонент | Готовность | Оценка |
|---|---:|---|
| Core Backend | 84% | Хорошее разделение handler/service/repository, Fx DI, recovery, события, транзакционные сценарии; остаются крупные сервисы и неодинаковая типизация ошибок |
| Authentication/RBAC | 82% | Argon2id, JWT, refresh rotation, блокировка пользователей, granular REST RBAC и route tests; нет MFA/self-service recovery, токены UI хранятся в localStorage |
| Agent | 77% | Реальные systemd/package/filesystem/archive/firewall/stats операции, bounded executor и Unix socket; остаются TOCTOU и общий CommandExecute |
| gRPC API | 76% | 34 типизированных RPC, streaming и recovery interceptors; upload RPC не реализован, нет capability/version negotiation |
| REST API | 79% | Версионированные маршруты, structured errors, permissions, pagination для users/tasks; нет OpenAPI и единой query-модели |
| Frontend application | 77% | 10 маршрутов, Pinia, i18n, responsive layout, reconnect/polling; практически отсутствуют unit/component/E2E тесты |
| UI/UX | 75% | Сильные Files, Firewall, Nginx и Fail2Ban workflow; остаются `alert`/`prompt`, неполная permission-aware логика и accessibility |
| Dashboard/processes | 80% | Снимки, live WebSocket, процессы и kill; нет истории/алертов/exporter/SLO |
| Sites/File Manager | 82% | Реальный site lifecycle, compensation, editor, batch, archives, ownership/permissions; отсутствует upload и backup/versioning |
| Services/logs | 74% | Управление systemd и streaming; JWT в URL streaming-соединения, ограниченный поиск/экспорт/retention |
| Users/audit | 75% | CRUD, роли, блокировка, смена пароля, история пользователя; нет глобального audit UI, MFA и делегированных custom roles |
| Task Manager | 72% | Персистентный список, cancel/retry/delete, recovery interrupted jobs, UI; выполнение process-local, retry только install/uninstall |
| Installer | 79% | amd64/arm64, apt/dnf detection, checksum, archive allowlist, systemd hardening; нет distro matrix, uninstall path defect и строгого failure gate запуска |
| Updater | 57% | Проверка версий, restricted request, canonical remote/clean/ff-only dev flow; нет подписей, pinning и автоматического rollback |
| CI/CD | 84% | Linux race, lint, security, amd64/arm64 release, nightly и реальный Ubuntu smoke; npm audit не блокирует, нет coverage/E2E/distro matrix |
| Тестовая база | 63% | Хорошие security/rollback tests в критичных местах и smoke; низкая плотность модульных/frontend/API contract тестов |
| Документация | 69% | Есть архитектура, API, install, security, testing и module audit; несколько документов и version labels устарели |

## 5. Архитектура Backend

### Реализовано хорошо

- Core запускается без root и общается с привилегированным Agent через локальный
  Unix socket.
- Fx-композиция отделяет домены auth, modules, sites, services, dashboard,
  settings, updater и platform adapters.
- SQLite-репозитории существуют для пользователей, сессий, настроек, аудита,
  модулей, сайтов, служб, снимков и задач.
- HTTP и gRPC имеют panic recovery; наружу не выдаётся panic value.
- Событийная шина изолирует panic подписчика и продолжает fan-out.
- Site lifecycle выполняет обязательные операции синхронно и компенсирует
  применённую Nginx-конфигурацию при ошибке persistence.
- Persisted pending/running tasks после рестарта явно переводятся в error, а не
  остаются «вечными».
- Реальный IP берётся от peer, непроверенные proxy headers не считаются
  доверенными.

### Технический долг

- Крупные файлы остаются точками высокой связности: `pkg/contract/module.go`,
  Agent client/server, stats collector, auth service, module handler, service
  manager, Fail2Ban.
- Ошибки модульных providers часто возвращаются как plain `error`, из-за чего
  пользовательская validation error может превратиться в HTTP 500.
- In-memory Event Bus не даёт гарантии доставки после commit; для обязательных
  downstream-процессов нужен transactional outbox.
- SQLite подходит одному хосту, но отсутствуют load/retention показатели для
  audit, jobs, snapshots и интенсивного polling.
- Миграционная политика неоднородна: ранние down migration пусты, более новые
  частично реализованы. Нет документированного restore drill.

## 6. Authentication, RBAC и безопасность Core

Закрыт один из главных рисков предыдущего состояния: generic module routes,
DataGrid, settings, logs, actions и custom module routes теперь защищены
`module:view` либо `module:configure`; это подтверждено route-level тестом.
Sites, services, users, settings и updater используют отдельные permissions.

Сильные стороны:

- Argon2id password hashing;
- обязательный сильный initial admin password;
- access/refresh token pair и rotation;
- session invalidation при logout/password change;
- роли admin/operator/viewer;
- CSRF handshake для cookie-sensitive запросов, Bearer API освобождён от
  лишней CSRF-проверки;
- same-origin ticket для dashboard WebSocket;
- rate limiting по прямому peer IP.

Оставшиеся риски:

- access и refresh token сохраняются в `localStorage`; XSS получает обе сессии;
- нет MFA, recovery codes, password reset и re-authentication для особо опасных
  операций;
- UI учитывает adminOnly только для Users; остальные элементы не моделируют
  granular permissions оператора/viewer;
- нет session/device UI и принудительного завершения отдельной сессии;
- глобальный security-header/CSP профиль и reverse-proxy TLS acceptance tests не
  подтверждены отдельной проверкой.

## 7. Agent и привилегированная граница

Agent реализует systemd, package management, filesystem, archives, UFW,
statistics, process control, logs и ограниченный command executor. Команды
запускаются без shell parsing, с чистым environment, timeout и bounded output.
Разрешённые аргументы и пути заметно уже первоначального варианта.

Файловый слой:

- использует allowed roots;
- проверяет границы корня;
- отклоняет symlink escape для проверяемых путей;
- не разрешает удаление самого managed root;
- запрещает special permission bits;
- применяет atomic replacement.

Архивы извлекаются in-process с проверкой traversal, типов записей, symlinks,
количества и размера. Это существенное улучшение.

Остаётся:

- строгая защита на базе directory descriptors/openat2 нужна для устранения
  symlink race между validation и operation;
- `CommandExecute` остаётся универсальным RPC и расширяет blast radius каждой
  ошибки allowlist;
- package/service allowlist не является политикой конкретного module ownership;
- root Agent systemd unit имеет ограниченное hardening по сравнению с Core;
- нет per-RPC audit/correlation ID и capability negotiation;
- `FileUploadStream` возвращает `Unimplemented`.

## 8. gRPC API

Контракт содержит 34 RPC:

- service actions/status/log streams;
- package install/remove/update/status;
- filesystem CRUD, ownership, archives и upload stream;
- typed Nginx site apply;
- UFW CRUD/status/toggle/reset;
- stats, processes, command execution и ping.

Плюсы:

- protobuf типизирует привилегированную границу;
- есть unary/stream panic recovery;
- Nginx site apply выделен в отдельную операцию;
- Core client инкапсулирует generated API.

Минусы:

- upload объявлен в protobuf и клиентском surface, но сервер возвращает
  `codes.Unimplemented`;
- нет server capabilities/minimum compatible version;
- нет idempotency key для mutating RPC;
- нет единого transaction/snapshot API для multi-file конфигураций;
- local socket security зависит от корректных filesystem permissions и
  systemd setup; remote/mTLS режим отсутствует.

## 9. REST API

REST API стал существенно полнее: auth, users, modules, module pages/actions,
Fail2Ban presets, jobs/tasks, dashboard, processes, sites/files, services,
settings и updater покрыты отдельными permissions. Users и tasks имеют
pagination/filtering.

Проблемы:

- `API.md` описывает лишь часть примерно 76 маршрутов;
- нет OpenAPI/JSON Schema и CI contract validation;
- naming неоднороден (`jobs/{id}` и `tasks`);
- pagination/filter/sort conventions различаются по доменам;
- нет idempotency для POST/PUT и optimistic concurrency/ETag;
- нет request body limit/versioned compatibility policy;
- response/error helpers продублированы в нескольких пакетах.

## 10. Frontend и UI

### Реально доступные разделы

- Login;
- Dashboard;
- Modules и Module Details;
- Sites с File Manager;
- Services и логи;
- Tasks;
- Processes;
- Users;
- Firewall;
- Settings/Updater.

Frontend использует Vue 3, Vite, Pinia, vue-i18n, TailwindCSS и lazy-loaded
routes. Production build разделён на chunks; крупнейший vendor chunk остаётся
разумным для локальной панели.

Сильные workflow:

- File Manager разбит на coordinator, table, toolbar, context menu, editor и
  composables;
- Nginx предоставляет status/health/settings/sites/certificates/configuration;
- Fail2Ban имеет информативные preset cards, details/settings/preview/rollback;
- Firewall имеет специализированный интерфейс правил;
- Users и Tasks имеют filter/pagination/modal details;
- mobile sidebar, RU/EN и dark/light темы реализованы.

UX-долг:

- generic DataGrid, Processes, Settings и Firewall всё ещё используют
  `alert()`; File Manager и password change используют `prompt()`;
- версия в sidebar захардкожена как `v1.0.0`, хотя проект pre-release;
- service log WebSocket передаёт JWT в `?token=...`, несмотря на ticket-based
  решение dashboard;
- нет глобальной toast/error boundary системы;
- permission-aware UI ограничен проверкой роли admin для Users;
- accessibility покрыта точечно, но нет keyboard/focus/modal audit;
- нет upload UI, хотя File Manager визуально близок к полноценному;
- polling Tasks/Processes/Logs не объединён в общий lifecycle/backoff слой.

## 11. Installer

Production installer поддерживает amd64/arm64, распознаёт apt/dnf families,
проверяет зависимости, скачивает release archive и `checksums.txt`, проверяет
SHA-256, разрешает только три ожидаемых binary entry и устанавливает Core,
Agent, CLI и systemd units. JWT secret генерируется автоматически.

Core unit имеет `NoNewPrivileges`, `ProtectSystem=strict`, `PrivateTmp` и
`ProtectHome`. Agent работает как root, что соответствует назначению, но
hardening ограничен.

Найденные дефекты:

- checksum защищает от случайной порчи, но checksum и archive публикуются в
  одном доверительном домене без подписи/provenance;
- installer поддерживает dnf декларативно, но CI проверяет только Ubuntu;
- ошибки `systemctl is-active` печатаются, но не всегда прерывают успешное
  завершение installer;
- `deployments/uninstall.sh` удаляет `/usr/local/bin/...`, тогда как installer
  устанавливает `/usr/bin/...`; обычная деинсталляция оставляет binaries;
- нет pre-upgrade backup и автоматического restore;
- нет clean-host tests для Debian/RHEL-family и upgrade-from-previous-version.

## 12. Updater

Updater имеет два режима:

- Core проверяет GitHub release/main и через Agent создаёт root-owned
  `update.request`;
- CLI/systemd path применяет update;
- dev source update проверяет canonical remote, `main`, чистый worktree и
  выполняет fast-forward-only merge.

Главный production-блокер: CLI каждый раз скачивает
`https://raw.githubusercontent.com/anrted/opendeploy/main/install.sh` и запускает
его от root. Скрипт не привязан к версии, которую пользователь видел в UI, и сам
installer/checksum не подписаны независимым ключом. Компрометация main или
release publishing chain превращается в root code execution.

Также отсутствуют:

- pinning tag/commit/digest;
- Sigstore/GPG verification;
- transactional binary swap;
- health gate с автоматическим rollback;
- совместимость schema downgrade;
- UI истории обновлений и диагностики неудачи.

## 13. CI/CD и релизы

CI запускает:

- golangci-lint;
- `go test -race ./...`;
- ESLint;
- Vitest;
- govulncheck;
- npm audit;
- Ubuntu install/provision/restart smoke.

Build workflow собирает linux amd64 и arm64, создаёт SHA-256 checksums и GitHub
release. Nightly и tag release workflows существуют.

Недостатки:

- `npm audit || true` делает frontend dependency findings информационными;
- golangci-lint устанавливается как `@latest`, что снижает воспроизводимость;
- GitHub Actions закреплены по floating major tag, а не commit SHA;
- нет SLSA provenance/signature/SBOM;
- нет coverage threshold;
- нет browser E2E;
- нет Debian/RHEL matrix;
- release workflow не требует прохождения отдельного release gate на том же
  commit;
- Node 20 runtime deprecation у используемых actions уже создаёт annotations.

## 14. Тесты

Лучшее покрытие сосредоточено в действительно рискованных областях:

- auth/CSRF/RBAC;
- WebSocket tickets;
- Agent executor operands/timeouts/output;
- filesystem root/symlink/permissions;
- archive traversal/symlink;
- config rollback;
- site compensation;
- module restart recovery;
- updater version comparison/request file;
- Nginx transactional settings/config/site behavior;
- Fail2Ban filters, presets, validation и rollback.

Пробелы:

- frontend имеет только три теста confirm store;
- нет E2E browser tests;
- Apache, Certbot, Firewall, Git, MySQL, Node.js, PHP и PostgreSQL почти не имеют
  собственных тестов;
- нет contract tests для всех 34 Agent RPC;
- нет REST route matrix по всем ролям и всем примерно 76 endpoint;
- нет installer upgrade/uninstall test;
- нет updater rollback/supply-chain tests;
- нет performance, soak, large-directory/log-stream tests;
- coverage не измеряется в CI.

## 15. Документация

Документационный набор широкий: README, Architecture, API, Install, Testing,
Security, Roadmap, Changelog, Stage 1 и отдельные Nginx/Fail2Ban audits.

Требуют актуализации после этого аудита:

- README всё ещё называет Users незавершёнными и занижает ряд модулей;
- ROADMAP ссылается на состояние 2026-07-27 и включает уже выполненные Users и
  часть Tasks;
- TESTING утверждает, что frontend lint проверяет formatting, и описывает старый
  Windows defect;
- API не содержит Users, Files, Services, Tasks, Presets и большинство module
  endpoints;
- CLI usage обещает `status/modules/sites/services`, но фактически полезными
  являются в основном version/update;
- отдельные generated/older comments содержат mojibake;
- нет operational runbook, backup/restore guide, threat model и compatibility
  policy.

## 16. Аудит встроенных модулей

| Модуль | Готовность | Реально реализовано | Главный долг |
|---|---:|---|---|
| Nginx | 87% | Lifecycle, rich status/health, sites, certificates metadata/renew, configuration explorer/editor, settings, validation и rollback | Нет push log streaming, полноценного certificate lifecycle, HTTP/3/capability model |
| Fail2Ban | 83% | Live jails/IP, permanent ban, 5 presets, details/settings/preview, filter expansion, validation и rollback | Общий `SettingsSchema` ведёт в no-op `SaveSettings`; IPv6/email/global config не завершены |
| Firewall | 79% | UFW status, CRUD, toggle/reset, IPv4/IPv6 UI, critical ports | Ограниченные tests, anti-lockout policy фиксирована и не учитывает custom SSH/panel ports |
| Certbot | 59% | Package/timer lifecycle, ACME webroot issue, интеграция с site SSL, Nginx certificate metadata/renew | Нет inventory/revoke/replace UI, account/email management и failure diagnostics |
| Apache | 47% | Package/service lifecycle и базовый vhost apply | Нет `apachectl configtest`, rollback, DataGrid/config UI, безопасной domain/path validation |
| PHP | 49% | Package/service foundation и pool helpers | Нет полного multi-version install, pool management UI/API и integration tests |
| Git | 38% | Package lifecycle/status foundation | Нет repositories, deploy keys, credential model, release deployment и rollback |
| Node.js | 36% | Package lifecycle/status foundation | Нет version manager, process manager, per-site runtime, npm/deploy workflow |
| MySQL | 39% | Package lifecycle и database/user helper | Нет production UI/API, backup/restore; SQL строится строками и требует строгой identifier/value модели |
| PostgreSQL | 27% | Package/service lifecycle | Нет database/user CRUD, credentials, backup/restore и UI |

### Особые выводы по модулям

- Nginx и Fail2Ban — самые зрелые и подтверждены unit плюс Ubuntu smoke.
- Firewall полезен как alpha, но опасные изменения должны иметь более сильный
  lockout preview.
- Certbot функционально работает через site lifecycle, однако отдельный module
  page не отражает весь certificate domain.
- Apache нельзя считать равноценным Nginx: конфигурация применяется без
  обязательного configtest/rollback.
- PHP, Git, Node.js, MySQL и PostgreSQL зарегистрированы как модули, но их
  наличие в каталоге не означает готовый product workflow.

## 17. Подтверждённые production-блокеры

### P0

1. **Непривязанная к релизу root update chain.** Updater скачивает и запускает
   текущий `main/install.sh`; нет независимой подписи, commit/tag pinning и
   rollback.
2. **Нет штатного backup/restore.** Перед update, migration, site/database
   изменениями отсутствует единый проверенный recovery workflow.
3. **JWT exposure в UI.** Access и refresh tokens лежат в localStorage, а
   service-log streaming передаёт token в URL.
4. **Не завершена строгая Agent filesystem isolation.** Нужна directory-FD
   защита от TOCTOU для root filesystem operations.
5. **Нет обязательного release acceptance matrix.** Подтверждена Ubuntu, но
   installer заявляет также Debian/RHEL-family; upgrade/uninstall не покрыты.
6. **Критичный installer/uninstaller defect.** Uninstaller удаляет другой binary
   prefix и может оставить рабочие root-capable binaries после «удаления».

### P1

7. Реализовать upload RPC/UI с лимитами, streaming checksum и atomic commit либо
   удалить ложный контракт до готовности.
8. Заменить no-op global Fail2Ban settings реальной transactional реализацией
   или убрать неработающую страницу.
9. Ввести OpenAPI и contract tests для REST/gRPC compatibility.
10. Добавить frontend component/E2E suite и accessibility gate.
11. Сделать npm audit блокирующим по политике severity/exceptions.
12. Добавить глобальный audit log UI, retention/export и correlation IDs.
13. Расширить durable task execution: idempotency, recovery/resume и retry
    support beyond module install/uninstall.
14. Разделить крупные Agent/Core/contracts файлы по доменам без изменения API.

## 18. Новый приоритетный backlog

### P0 — до beta

- Перепроектировать updater: release manifest, pinned digest, Sigstore/GPG,
  staging install, health check и автоматический rollback.
- Реализовать backup/restore OpenDeploy DB/config, sites и certificates;
  добавить restore drill в CI.
- Перевести web auth на защищённую cookie/session модель либо memory access token
  + HttpOnly rotating refresh cookie; убрать JWT из WebSocket URL через tickets.
- Исправить uninstall prefix и сделать service startup failure фатальным для
  installer.
- Добавить directory-FD/openat2 containment для Agent filesystem mutations.
- Добавить clean install/upgrade/uninstall matrix минимум Ubuntu LTS + Debian;
  либо официально сузить заявленную поддержку.
- Зафиксировать release dependencies/actions по версиям/commit и публиковать
  SBOM/provenance.

### P1 — beta quality

- Upload в File Manager и безопасные large-file limits.
- Полный certificate center: inventory, expiry, issue, renew, revoke, replace.
- Реальные global Fail2Ban settings и завершённый notification/IPv6 model.
- Apache configtest/rollback либо явное experimental состояние.
- Browser E2E для login/users/sites/files/Nginx/Firewall/Fail2Ban/tasks/update
  preview.
- OpenAPI generation и единые pagination/sort/error/idempotency conventions.
- Permission-aware UI для admin/operator/viewer.
- Global toast/modal forms вместо alert/prompt.
- Audit explorer, retention, export и request/job correlation.
- Метрики Prometheus/OpenTelemetry, readiness, alerting и SLO.
- Coverage thresholds и module-specific integration tests.

### P2 — после beta

- MySQL production workflow с безопасным SQL, credentials и backup/restore.
- PHP version/pool management.
- Node.js runtime/process/deployment management.
- Git deploy keys, atomic releases и rollback.
- PostgreSQL database management.
- MFA, session/device management и self-service password recovery.
- Durable outbox/worker model для гарантированных фоновых процессов.

### P3 — v2

- Remote Agent с mTLS enrollment/rotation.
- Multi-host inventory и orchestration.
- Tenant/organization boundaries.
- Signed third-party modules и compatibility sandbox.
- HA control plane и optional PostgreSQL storage.

## 19. Рекомендуемые критерии beta

Beta допустима, когда:

- все P0 закрыты regression/integration tests;
- update можно безопасно откатить после failed health check;
- backup восстанавливается на чистом хосте;
- токены не попадают в localStorage/URL;
- installer upgrade/uninstall проверены на поддерживаемых дистрибутивах;
- Nginx, Sites/Files, Services, Users, Firewall, Fail2Ban и Certificates имеют
  browser E2E;
- API schema генерируется или валидируется в CI;
- frontend dependency security gate обязателен.

## 20. Итоговая рекомендация

OpenDeploy можно использовать для разработки, демонстрации и контролируемого
тестового Linux-хоста в доверенной сети. Проект уже достаточно зрел для
последовательного hardening к beta и не является прототипом.

До закрытия P0 не рекомендуется:

- выставлять Core напрямую в Интернет;
- использовать updater без внешней проверки и резервной копии;
- считать Git/MySQL/PostgreSQL/PHP/Node.js завершёнными workflow;
- использовать OpenDeploy как единственную точку восстановления production
  сервера.

Рекомендуемая маркировка релизов остаётся **alpha**. Следующий инженерный этап —
не расширение каталога модулей, а завершение доверенной update/recovery chain,
auth token hardening, supported-distro evidence и end-to-end тестирование уже
существующих сильных функций.
