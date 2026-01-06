# Windows (Docker Desktop + WSL2): настройка и workflow

Мы запускаем всё внутри WSL2 для консистентных Linux-инструментов (docker CLI, файловые бэкенды).

## Короткие правила

1. Запускайте эксперименты в **файловой системе WSL** (например, `~/taidon-ws/exp01`), а не в `/mnt/c/...`.
2. Выполняйте команды в **Ubuntu (WSL) терминале**.
3. Docker должен быть доступен внутри WSL:
   - установлен Docker Desktop
   - включена интеграция WSL для вашей дистрибуции

Проверьте внутри WSL:

```bash
docker version
docker ps
```

## Типовой workflow (внутри WSL)

```bash
cd ~/taidon-repo
pnpm install

mkdir -p ~/taidon-ws/exp01
pnpm bench -- --workspace ~/taidon-ws/exp01 --examples chinook --storages plain --snapshots off
```

## Запуск из PowerShell (опционально)

Можно запускать те же команды через хелпер, который выполняет их в WSL:

```ps
node scripts/wsl-run.mjs --distro Ubuntu -- pnpm bench -- --workspace ~/taidon-ws/exp01 --examples chinook --storages plain,btrfs
```
