# Компонентная структура prepare job events

Этот документ фиксирует внутреннюю структуру, необходимую для поддержки
events-first мониторинга в CLI и engine.

## CLI (frontend/cli-go)

Модули:

- `internal/client`
  - HTTP-хелперы запросов и типизированное JSON-декодирование.
  - Reader для events stream, который:
    - принимает `events_url`,
    - поддерживает `Range: events=...` при reconnect,
    - отдает `PrepareJobEvent` как итератор/канал.

- `internal/cli`
  - `waitForPrepare` работает в events-first режиме:
    - открывает stream, парсит NDJSON,
    - на `status` событиях повторно запрашивает job status,
    - останавливается на `succeeded`/`failed`,
    - возвращает ошибку, если stream завершился без terminal status.
  - Интерактивное управление watch:
    - `Ctrl+C` открывает control prompt (`stop`, `detach`, `continue`),
    - `stop` запрашивает подтверждение перед отменой,
    - повторный `Ctrl+C` в открытом prompt трактуется как `continue`,
    - в composite `prepare ... run ...` действие `detach` пропускает
      последующую фазу `run`.
  - Рендеринг:
    - обновление строки статуса по новым событиям,
    - анимация спиннера для повторяющихся событий,
    - отображение `message` события (если есть),
    - в verbose-режиме печать каждого события в отдельной строке.

- `internal/app`
  - структурных изменений нет, кроме использования обновленного поведения
    watch/detach/stop.

Data ownership:

- Состояние stream (последний event index, состояние prompt/spinner) хранится
  только в памяти.
- Финальный результат job читается из `GET /v1/prepare-jobs/{jobId}`.

## Engine (backend/local-engine-go)

Модули:

- `internal/httpapi`
  - `/v1/prepare-jobs/{jobId}/events` поддерживает:
    - range parsing для `Range: events=...` (опционально),
    - `206 Partial Content` с `Content-Range: events`,
    - `Accept-Ranges: events` при поддержке.
  - endpoint отмены для queued/running prepare jobs.

- `internal/prepare`
  - Event bus и storage уже существуют.
  - Опциональная range-поддержка читает события из queue по event index.
  - Публикует `log` события runtime/DBMS операций и heartbeat task events
    (~500ms), когда новых событий нет.
  - Отмена кодируется как `failed` у job/task с `error.code=cancelled`.

Data ownership:

- События хранятся в prepare queue (SQLite) и стримятся оттуда.
- Streaming stateless, кроме запрошенного `events` range.

## Deployment Units

- CLI: читает события, валидирует terminal status, управляет attach/watch UX.
- Engine: производит и хранит события, поддерживает streaming и cancel.
