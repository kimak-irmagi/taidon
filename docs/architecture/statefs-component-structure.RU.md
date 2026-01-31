# Структура компонента StateFS

Этот документ описывает компонент `statefs`, который инкапсулирует всю файловую логику и снапшотирование внутри engine. Остальные модули не должны зависеть от конкретной ФС.

## 1. Цели

- Изолировать **ФС-специфичное** поведение (btrfs/overlay/copy) за одним контрактом.
- Централизовать **валидацию стора** (маунты, тип ФС, корректность subvolume).
- Владеть **структурой путей** (base/state/runtime/job runtime).
- Скрыть детали тактики снапшотов от prepare/run/deletion.

## 2. Ответственности

- Детектировать/валидировать state store root и состояние маунта.
- Выбирать backend (btrfs/overlay/copy) по конфигу и возможностям ФС.
- Предоставлять функции для вычисления путей base/states/runtime/job runtime.
- Создавать директории/subvolume с учетом backend.
- Делать clone/snapshot.
- Удалять пути безопасно (например, btrfs subvolume delete перед `rm -rf`).
- Экспортировать capabilities (например, нужен ли стоп БД).

## 3. Контракт (черновик)

```go
// Package statefs

type Capabilities struct {
    RequiresDBStop        bool
    SupportsWritableClone bool
    SupportsSendReceive   bool
}

type CloneResult struct {
    MountDir string
    Cleanup  func() error
}

type StateFS interface {
    Kind() string
    Capabilities() Capabilities

    // Валидация
    Validate(root string) error

    // Структура путей
    BaseDir(root, imageID string) (string, error)
    StatesDir(root, imageID string) (string, error)
    StateDir(root, imageID, stateID string) (string, error)
    JobRuntimeDir(root, jobID string) (string, error)

    // Операции с хранилищем
    EnsureBaseDir(ctx context.Context, baseDir string) error
    EnsureStateDir(ctx context.Context, stateDir string) error
    Clone(ctx context.Context, srcDir, destDir string) (CloneResult, error)
    Snapshot(ctx context.Context, srcDir, destDir string) error
    RemovePath(ctx context.Context, path string) error
}
```

Примечания:
- `RemovePath` инкапсулирует ФС-специфичную очистку: btrfs удаляет subvolume, copy использует `os.RemoveAll`.
- `Validate` включает проверки маунта и типа ФС (например, btrfs действительно смонтирован и видим engine).
- Структура путей контролируется тут, чтобы backend мог менять layout без правок prepare/deletion.

## 4. Точки интеграции

- `prepare.Manager`
  - использует `StateFS.Validate` перед выполнением
  - использует `StateFS.EnsureBaseDir`/`EnsureStateDir`
  - использует `Clone`/`Snapshot` при построении state
  - использует `JobRuntimeDir` для runtime каталога
- `deletion.Manager`
  - использует `RemovePath` для runtime-директорий
- `run.Manager`
  - использует пути `StateFS` при восстановлении контейнеров

## 5. Размещение пакета

- Новый пакет: `internal/statefs`
- `internal/snapshot` становится внутренней частью `statefs` или полностью заменяется.

## 6. Открытые вопросы

- Нужен ли отдельный объект `PathLayout` вместо набора функций?
- Нужны ли отдельные методы (`RemoveRuntimeDir`, `RemoveStateDir`) или достаточно `RemovePath`?
