# Workflow экспериментов

## Структура workspace

Мы запускаем эксперименты во внешнем рабочем каталоге (не обязательно внутри репозитория).

Рекомендуемая структура:

- `<workspace>/dist/bin/sqlrs` (или `sqlrs.cmd`) - собранный CLI
- `<workspace>/sqlrs-work/` - артефакты запуска (метрики, логи)
- `<workspace>/results/` - логи экспериментов и агрегированные результаты

## Сборка CLI в workspace

```bash
pnpm build:cli -- <workspace-path>
```

Пример:

```bash
pnpm build:cli -- ~/taidon-ws/exp01
```

## Запуск одного примера

```bash
pnpm run:one -- \
  --workspace ~/taidon-ws/exp01 \
  --example chinook \
  --storage plain \
  --snapshots off
```

## Запуск матричного бенчмарка

```bash
pnpm bench -- \
  --workspace ~/taidon-ws/exp01 \
  --examples chinook,sakila,postgrespro-demo \
  --storages plain,btrfs,zfs \
  --snapshots off,stop
```

`bench` сохраняет логи каждого прогона в `<workspace>/results/:`, а `sqlrs` пишет метрики в `<workspace>/sqlrs-work/runs/<run_id>/metrics.json`.
