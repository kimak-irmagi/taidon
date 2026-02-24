# Контроль Емкости State Cache (Local MVP Hardening)

Этот документ фиксирует дизайн ограниченного (bounded) кэша снапшотов состояний
для локального engine. Он дополняет:

- [`state-cache-design.RU.md`](state-cache-design.RU.md)
- [`runtime-snapshotting.RU.md`](runtime-snapshotting.RU.md)
- [`statefs-component-structure.RU.md`](statefs-component-structure.RU.md)
- [`engine-internals.RU.md`](engine-internals.RU.md)

## 1. Проблема

Сейчас локальный state cache хранит снапшоты неограниченно долго, если их явно
не удалять вручную. Это приводит к неограниченному росту `<state-store-root>`,
что операционно небезопасно для долгоживущих workspace и CI-агентов.

## 2. Поведение Для Пользователя (Логический Контракт)

Кэш должен вести себя предсказуемо:

1. Оператор задает единый логический бюджет кэша.
2. Эффективный cache max привязан к емкости файловой системы state-store.
3. Когда usage пересекает high watermark, автоматически запускается eviction.
4. Eviction также запускается при падении свободного места ниже reserve floor.
5. Eviction продолжается, пока usage не станет ниже low watermark и свободное
   место не вернется выше reserve.
6. Состояния, которые активно нужны системе, не удаляются обычной политикой.
7. Если освободить достаточно места нельзя, prepare завершается явной и
   диагностируемой ошибкой.

Это дает строгую гарантию емкости, а не best-effort cleanup.

## 3. Контракт Конфигурации

Лимиты емкости публикуются через существующий server config API (`sqlrs config`).

Планируемые пути:

- `cache.capacity.maxBytes`: integer, nullable
  - `null` или `0` означает "использовать только store-coupled max".
- `cache.capacity.reserveBytes`: integer, nullable
  - если `null`, резерв по умолчанию `max(10 GiB, 10% от общей емкости store)`.
- `cache.capacity.highWatermark`: число в `(0,1]`, по умолчанию `0.90`
- `cache.capacity.lowWatermark`: число в `(0, high)`, по умолчанию `0.80`
- `cache.capacity.minStateAge`: duration string, по умолчанию `10m`

Расчет эффективного лимита:

1. `store_total_bytes` измеряется у ФС, где расположен state store.
2. `effective_max_from_store = max(0, store_total_bytes - reserveBytes)`.
3. Если `cache.capacity.maxBytes > 0`, то
   `effective_max = min(maxBytes, effective_max_from_store)`.
4. Иначе `effective_max = effective_max_from_store`.

Примечания:

- Для `--store image|device` (btrfs VHDX/device) `store_total_bytes` - емкость
  смонтированного виртуального диска.
- Для `--store dir` (copy/overlay/root btrfs) `store_total_bytes` - емкость
  host ФС в точке маунта store.

Правила валидации:

- `maxBytes >= 0`
- `reserveBytes >= 0`
- `0 < lowWatermark < highWatermark <= 1`
- `minStateAge >= 0`

## 4. Правила Допуска К Eviction

State считается кандидатом только если выполняются все условия:

1. `refcount == 0` (ни один активный instance его не использует)
2. нет потомков (`leaf` в DAG состояний)
3. возраст state >= `minStateAge`
4. state не защищен retention-метаданными (будущая pin/class политика)

Это сохраняет корректность и независимость от конкретного backend.

## 5. Политика Выбора (v1)

В v1 приоритет - объяснимость, а не сложный скоринг:

1. Собрать набор кандидатов по правилам допуска.
2. Отсортировать:
   - сначала более старые по `last_used_at`,
   - затем более крупные по `size_bytes`.
3. Удалять по порядку, пока usage <= low watermark.

В будущих версиях можно добавить replay-cost/priority scoring, но v1 должен
оставаться прозрачным по логам.

## 6. Триггеры И Модель Выполнения

Eviction запускается по триггерам:

1. после успешного создания state (`Snapshot` + `CreateState` commit),
2. при startup recovery engine,
3. перед `state_execute` и перед `Snapshot`, когда strict capacity mode включен,
4. опционально периодическим фоновым циклом.

Выполнение сериализуется глобальным evictor lock
(`.evict.lock` в state-store root), чтобы избежать гонок.

## 7. Обязанности Backend Во Всех Snapshot-Механиках

Политика едина для `copy`, `overlayfs`, `btrfs`; backend-специфика ограничена
учетом занимаемого места и примитивом удаления.

### 7.1 Измерение usage

- `copy` / `overlayfs`: рекурсивный подсчет размера директории.
- `btrfs`: предпочтительно оценка через `btrfs filesystem du`; fallback на
  рекурсивный подсчет.
- будущий `zfs`: нативные метрики used-space на уровне dataset.

Evictor дополнительно измеряет:

- `store_total_bytes`
- `store_free_bytes`

Evictor обязательно логирует:

- ожидаемое освобождение по метаданным кандидата,
- фактический usage store до/после.

### 7.2 Примитив удаления

Физическое удаление выполняется через существующую семантику удаления
(statefs-aware remove), сохраняя backend safety behavior:

- btrfs subvolume delete перед удалением директории,
- overlay/copy fallback-логика удаления путей.

## 8. Изменения Метаданных И Схемы Хранилища

Метаданные state расширяются для детерминированных решений политики:

- `last_used_at` (timestamp)
- `use_count` (integer)
- `min_retention_until` (timestamp, nullable)
- `evicted_at` (timestamp, nullable, опционально для tombstones)
- `eviction_reason` (text, nullable, для диагностики)

Поле `size_bytes` уже существует и остается основным сигналом размера state.

Для corner-case "даже один state не помещается" engine хранит скользящие
наблюдения required build space по `(image_id, prepare_kind)`:

- latest successful peak build bytes,
- minimum observed successful build bytes.

## 9. Интеграция Компонентов

### 9.1 Новый внутренний компонент

Добавляется `prepare.cacheEvictor` (или соседний пакет), который:

- читает capacity-конфиг,
- измеряет usage store,
- строит список кандидатов,
- удаляет состояния и публикует diagnostics/events.

### 9.2 Существующие компоненты

- `prepare.snapshotOrchestrator` триггерит eviction после commit снапшота.
- `deletion.Manager` остается источником истины по безопасному удалению state.
- `httpapi` и CLI diagnostics публикуют результаты eviction.

## 10. Семантика Ошибок

Когда capacity control включен, enforcement строгий:

1. если usage измерить нельзя, prepare падает с
   `cache_enforcement_unavailable`;
2. если eviction не может удовлетворить ограничениям (остатки
   protected/in-use/non-leaf), prepare падает с `cache_full_unreclaimable`;
3. если effective limit слишком мал, чтобы материализовать хотя бы один state,
   prepare падает с `cache_limit_too_small`.

`cache_full_unreclaimable` должен включать machine-readable reason, в том числе:

- `usage_above_high_watermark`
- `physical_free_below_reserve`

`cache_limit_too_small` должен включать:

- `effective_max_bytes`
- `observed_required_bytes` (если известен)
- `recommended_min_bytes` (оцененный нижний порог)

Если несмотря на preflight внутри исполнения/снапшота произошел `ENOSPC`, его
нужно нормализовать в одну из ошибок выше и добавить phase
(`prepare_step`, `snapshot`, `metadata_commit`).

Обе ошибки должны включать machine-readable детали (причины блокировки, сколько
байт не хватает, сколько реально можно освободить).

## 11. Наблюдаемость

Публикуются структурированные события/логи:

- `cache_check` (usage, thresholds, trigger)
- `cache_evict_candidate` (state_id, reason, size_bytes, rank signals)
- `cache_evict_result` (state_id, success/failure, bytes_before, bytes_after)
- `cache_evict_summary` (evicted_count, freed_bytes, blocked_count)

Минимум операторской видимости:

- текущий usage в байтах,
- настроенные max/high/low,
- summary последнего прогона eviction.

## 12. План Внедрения

1. Реализовать расширения config и schema.
2. Реализовать ядро HWM/LWM + eligibility policy.
3. Подключить триггер после snapshot commit.
4. Добавить startup-check и диагностику.
5. Добавить backend-specific usage estimators и кроссплатформенные тесты.

## 13. Что Не Входит В v1

- Глобально оптимальный eviction по всему DAG.
- Балансировка shared cache между workspace.
- Агрессивный override pin-защиты в обычном режиме.
