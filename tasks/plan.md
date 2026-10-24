# Implementation Plan: pdf-converter

## Overview

Construir `pdf-converter`, el wrapper de Pandoc que convierte Markdown a PDF al momento de la descarga (ADR-0005), en Go. Expone un único endpoint `POST /convert` (síncrono, sin cola) y es invocado solo por `pdf-main`. Ejecuta Pandoc con motor **Typst** leyendo de `stdin` y escribiendo a `stdout` (RAM-only, sin archivos temporales). Errores en RFC 9457 (422 "Invalid Markdown" / 500 "Conversion failed"), DTOs JSON `snake_case` con binario en base64, límite de entrada `MAX_FILE_SIZE_MB` (ADR-0008, 20 MB desde el `.env` raíz).

**Seguimiento:** las tareas de abajo son la fuente de las issues. El usuario las mapea 1:1 a GitHub Issues, agrupadas en **4 milestones, uno por fase**; cada milestone cierra en **un PR** con tantos commits como tareas.

## Architecture Decisions

1. **Router `chi/v5`** — aprobado en spec, coherente con `pdf-main`.
2. **Núcleo puro, sin acoplar al transporte** (ADR-0004) — `internal/converter` expone `Convert(ctx, timeout, markdown) ([]byte, error)`, sin importar DTOs ni HTTP. El handler es una capa delgada.
3. **Pandoc por pipes (`stdin`/`stdout`)** — `exec.CommandContext` con `cmd.Stdin`/`cmd.Stdout`/`cmd.Stderr`; nunca `os.CreateTemp`. El motor elegido (**Typst**, `--pdf-engine=typst`) renderiza en memoria, a diferencia de LaTeX que escribe `.aux`/`.log` en TMPDIR.
4. **Clasificación de errores del subproceso** — `ErrInvalidMarkdown` (parse, → 422) vs `ErrConversion` (fallo genérico del motor, → 500), con `title` estable. La distinción se basa en el `stderr`/exit de Pandoc.
5. **Límite de entrada `MAX_FILE_SIZE_MB`** — 1 MB = 1 048 576 bytes; default 20 (ADR-0008). Se aplica al byte-length de `content`. En el compose se inyecta desde el `.env` raíz (`${MAX_FILE_SIZE_MB:-20}`).
6. **Tests de integración en `/test`** — validan Typst sobre tablas/headers y la ausencia de archivos temporales; se omiten con `t.Skip` si `pandoc`/`typst` no están en PATH. El resto de tests son unit (no requieren Pandoc).
7. **Deploy como servicio interno** — `networks: [fast_pdf_network]`, **sin** `ports` al host, imagen multi-stage con `pandoc` + `typst` + `ca-certificates`, usuario non-root.

## Task List

### Milestone 1 — Foundation
- [ ] Task 1: Bootstrap del módulo Go + config desde env
- [ ] Task 2: DTOs del wire + helpers RFC 9457

### Checkpoint: Foundation
- [ ] `go build ./...` limpio; `go vet ./...` sin errores
- [ ] DTOs y problem alineados con el contrato

### Milestone 2 — Núcleo de conversión (RAM-only)
- [ ] Task 3: Núcleo `converter.Convert` (pandoc + typst, pipes)

### Checkpoint: Núcleo
- [ ] `Convert` produce PDF válido por pipes sin tocar disco

### Milestone 3 — Transporte HTTP
- [ ] Task 4: Router chi + handler `POST /convert` + `/health`

### Checkpoint: Transporte
- [ ] `POST /convert` end-to-end: 200 / 400 / 413 / 422 / 500

### Milestone 4 — Integración + wiring + deploy
- [ ] Task 5: Tests de integración en `/test`
- [ ] Task 6: `main.go` (wiring + graceful shutdown)
- [ ] Task 7: `Dockerfile` + `docker-compose.yml`

### Checkpoint: Complete
- [ ] Todos los success criteria del spec verificados
- [ ] `gofmt -l .` vacío; `go test ./...` y `go vet ./...` pasan
- [ ] Listo para REVIEW

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| `typst` no está en los repos de la imagen base (alpine) → instalación no trivial en el Dockerfile | High | Instalar el binario de Typst (release oficial) en la imagen final; fallback documentado: WeasyPrint |
| Distinguir "Markdown inválido" (422) de fallos genéricos de Pandoc (500) es frágil | Med | Clasificar por exit code / patrón de `stderr`; mantener `title` estable y tests que fijen ambos casos |
| Tests de conversión dependen de `pandoc`+`typst` en el entorno local/CI | Med | `t.Skip` si los binarios faltan; los unit tests no dependen de Pandoc |
| Overhead base64 del PDF en el wire (~33%) | Low | Ya asumido por ADR-0002/0008; el límite de entrada es sobre `content` (Markdown), no sobre el PDF de salida |

## Open Questions

- Ninguna pendiente de diseño. Queda una **issue en `pdf-main`** para propagar el `422` de Markdown inválido en lugar de un `503` genérico (fuera del alcance de este repo).
