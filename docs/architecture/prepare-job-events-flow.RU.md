# Поток событий prepare job

Этот документ описывает events-first флоу наблюдения за `prepare` job,
с минимальным polling и явным поведением reconnect.

## Interaction Flow

1) CLI отправляет job
   - `POST /v1/prepare-jobs`
   - Ответ содержит `job_id`, `status_url`, `events_url`

2) CLI открывает events stream
   - `GET {events_url}`
   - Ответ в формате NDJSON (`application/x-ndjson`)
   - CLI читает поток до завершения (см. правила completion ниже).

3) CLI рендерит прогресс
   - Каждое событие обновляет строку статуса.
   - Повторяющиеся события могут анимироваться спиннером без новых строк.
   - `log` события содержат сообщения runtime/DBMS операций (Docker/Postgres).
   - В verbose-режиме каждое событие печатается с новой строки.
   - В интерактивном режиме `Ctrl+C` открывает control prompt:
     - `stop` (с подтверждением),
     - `detach`,
     - `continue`.

4) Валидация статуса при status events
   - На любое `status` событие (queued/running/succeeded/failed)
     CLI вызывает `GET /v1/prepare-jobs/{jobId}`.
   - Если status endpoint возвращает `succeeded` или `failed`,
     CLI завершает наблюдение.

5) Действия из control prompt
   - `continue`: закрыть prompt и продолжить наблюдение.
   - `detach`: прекратить наблюдение и выйти, оставив job выполняться.
   - `stop`: запросить подтверждение; после подтверждения отправить запрос
     отмены и продолжить наблюдение до terminal status.

6) Завершение stream
   - Если HTTP status равен 4xx, CLI завершает команду с ошибкой.
   - Если есть `Content-Length`, stream считается завершенным после чтения
     всех объявленных байт.
   - Если stream завершился без определенного terminal status, CLI завершает
     команду с ошибкой.

7) Reconnect поведение (опционально)
   - При disconnect CLI возобновляет поток через `Range: events=...`.
   - Если сервер игнорирует range и отвечает 200, CLI читает поток сначала.

8) Heartbeat поведение
   - Пока task в статусе running, engine повторяет последнее task-событие
     с новым timestamp, если новых событий нет примерно 500ms.

## Completion Rules

- Success: статус job подтверждён как `succeeded`.
- Failure: статус job подтверждён как `failed`, или stream завершился без
  terminal status, или получен 4xx ответ.
- Отмена кодируется как `failed` с `error.code=cancelled`.

## Sequence Diagram (informal)

```text
CLI -> Engine: POST /v1/prepare-jobs
Engine -> CLI: 201 { job_id, events_url, status_url }
CLI -> Engine: GET events_url
Engine -> CLI: NDJSON stream of PrepareJobEvent
CLI -> Engine: GET /v1/prepare-jobs/{jobId} (on status events)
Engine -> CLI: PrepareJobStatus (succeeded|failed)
```
