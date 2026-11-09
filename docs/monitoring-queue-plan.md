# Мониторинг IONE

Proxmox — после этой модели. Не смешивать этапы.

## Цель

Тик по умолчанию (~3–5 мин) смотрит **только статусы**.

Клиент/админ меняет инстанс в NoCloud → в `data.pending_ops` кладётся **одна операция** (например `disk_resize`). Следующий тик делает только её.

ONE → NoCloud на дешёвом тике: power/state. Железо из Sunstone в биллинг **не** тащим.

---

## Сделано

A. PR #211 / восстановление живой VM:

- `auto_start=false` + живая VM: `vmid`/`vm_name` + `StatusesClient`, не PENDING с пустым meta
- `_patch` в ядре: `MERGE(stored, incoming)`
- instantiate дописывает vmid в существующий `data`
- неизвестный LCM = VM жива

A + 1–5 в коде (не задеплоено на nc2dev). Этап 6 Proxmox не делался.

---

## Не делать, пока не дошли по порядку

- очередь `pending_ops` до фикса публикаций и lock
- вырезать `CheckInstancesGroup` до явных create/delete
- Proxmox
- принимать CPU/диск из UI OpenNebula в billing
- широкий рефакторинг биллинга, IPAM, goca upgrade

---

## Порядок (обязательный)

### 1. Публикация data + один тик на SP — в работе

Иначе очередь и дешёвый тик бесполезны: stale snapshot затирает vmid/ops.

- [x] lock: не запускать новый Monitoring SP, пока предыдущий жив
- [x] биллинг: патч дат, не весь `data`
- [x] create: патч identity
- [x] invoke / прочие `POST_INST_DATA` полного снимка — по мере касания, не заранее

### 2. Не ломать create/status на ошибке соседа

- [x] `PrepareService` fail не `continue` весь IG
- [x] `StatusesClient`: сети из свежего `par.Meta`, не из `inst.State` запроса
- [x] `ReservePublicIP` не подменять новый `public_ip_pool` мёртвым parent user-vnet

### 3. Дешёвый тик (B)

Контракт одного Monitoring:

1. lock SP  
2. лёгкий список VM пользователя  
3. без ops: только state  
4. create: нет vmid + auto_start → PrepareService + instantiate  
5. delete: status DEL + есть VM → terminate  
6. billing dates — патч  
7. `MonitorLocation` — каждый N-й тик, не каждый status  

- [x] `CheckInstancesGroup` / `VMToInstance` / hash **не** на Valid VM
- [x] lock + `creating` чтобы два тика не плодили две VM

### 4. Очередь (C)

- [x] очередь при Update инстанса (diff resources)
- [x] worker на тике: `disk_resize` / `cpu` / `ram` / `ips`
- [x] create/delete явные
- [x] cpu/ram: POWEROFF на одном тике, resize на следующем
- [x] billing upgrade после `done`
- [x] страховка: `applied_spec` / `applied_hash`

### 5. Тесты сценариев (не только хелперы)

- два тика ≠ две VM  
- disk_resize не трогает CPU  
- billing snapshot не стирает vmid  
- IP fail на create не глушит status соседей  
- live VM без auto_start выходит из PENDING  
- IP в state из свежего meta  

### 6. Proxmox

Тот же контракт, отдельным этапом.

---

## Явные ловушки

| Если сделать так | Сломается |
|---|---|
| Вырезать CheckInstancesGroup сразу | не создаются и не удаляются VM |
| Очередь без `_patch` биллинга | ops/vmid затирает stale tick |
| Hash vs XML на каждом тике как «очередь» | снова полный reconcile + revert Sunstone |
| Upgrade billing до resize | списали, диск не вырос |
| `continue` после «Couldn't reserve Public IP» | нет status/billing у всей группы |

---

Этапы 1–5 в коде. Дальше: деплой IONE+ядро, проверка на dev, затем Proxmox.
