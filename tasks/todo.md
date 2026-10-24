# Tasks — pdf-converter

> Tareas ordenadas por dependencia, agrupadas en **4 milestones (uno por fase)**.
> Cada tarea mapea a una issue; cada milestone cierra en **un PR** con tantos
> commits como tareas.

## Milestone 1 — Foundation

## Task 1: Bootstrap del módulo Go + config desde env

**Description:** Inicializar el módulo Go (`github.com/Parse-Documents-Fast/pdf-converter`, go 1.26.8), fijar la dependencia `chi/v5` y crear el loader de configuración por variables de entorno.

**Acceptance criteria:**
- [ ] `go.mod` con módulo `github.com/Parse-Documents-Fast/pdf-converter` y `chi/v5` en `go.sum`
- [ ] `internal/config` lee `HTTP_ADDR` (`:8000`), `LOG_LEVEL` (`info`), `CONVERT_TIMEOUT` (`10s`) y `MAX_FILE_SIZE_MB` (`20`) con defaults del spec
- [ ] `MAX_FILE_SIZE_MB` sigue ADR-0008: 1 MB = 1 048 576 bytes; se expone el límite en bytes (20 971 520)
- [ ] Config expuesta como struct inmutable con un `Load()` testeable

**Verification:**
- [ ] `go build ./...` y `go vet ./...` pasan
- [ ] Test unitario de `Load()` con env y defaults

**Dependencies:** None

**Files likely touched:**
- `go.mod`, `go.sum`
- `internal/config/config.go`
- `internal/config/config_test.go`

**Estimated scope:** Small (2-3 files)

---

## Task 2: DTOs del wire + helpers RFC 9457

**Description:** Definir el DTO del contrato (`POST /convert`) con tags `snake_case` y los helpers de Problem Details (RFC 9457), espejo de `pdf-main`.

**Acceptance criteria:**
- [ ] `internal/dto` con `ConvertRequest{Content string}` y `ConvertResponse{ContentBase64 []byte, MimeType string}`
- [ ] Tags JSON `snake_case`: `content`, `content_base64`, `mime_type`
- [ ] `ContentBase64 []byte` se serializa a base64 en el wire (ADR-0002)
- [ ] `internal/problem` con `WriteProblem(w, status, title, detail, instance)` (`type: about:blank`)

**Verification:**
- [ ] `go test ./internal/dto/... ./internal/problem/...`
- [ ] El JSON producido coincide con el ejemplo del contrato (`content_base64` + `mime_type: application/pdf`)

**Dependencies:** Task 1

**Files likely touched:**
- `internal/dto/converter.go`, `internal/dto/dto_test.go`
- `internal/problem/problem.go`, `internal/problem/problem_test.go`

**Estimated scope:** Small (3-4 files)

---

## Milestone 2 — Núcleo de conversión (RAM-only)

## Task 3: Núcleo `converter.Convert` (pandoc + typst, pipes)

**Description:** Implementar la función pura que ejecuta Pandoc con Typst, leyendo de `stdin` y escribiendo a `stdout` (pipes), sin archivos temporales. Es el punto de mayor riesgo (motor + restricción RAM-only).

**Acceptance criteria:**
- [ ] `Convert(ctx, timeout, markdown string) ([]byte, error)` invoca `pandoc -f markdown -t pdf --pdf-engine=typst -o -`
- [ ] `cmd.Stdin`/`cmd.Stdout`/`cmd.Stderr` por pipes; **no** se crea ningún archivo temporal
- [ ] Timeout aplicado vía `context` (deadline derivado de `CONVERT_TIMEOUT`)
- [ ] En fallo (exit != 0) se propaga el `stderr` de Pandoc en el error
- [ ] Errores clasificados: parse de Markdown → `ErrInvalidMarkdown`; resto → `ErrConversion`

**Verification:**
- [ ] `go test ./internal/converter/...` (requiere `pandoc`+`typst` en PATH): pipe correcto, propagación de `stderr`, timeout, y `TMPDIR` controlado que queda vacío tras convertir

**Dependencies:** Task 1

**Files likely touched:**
- `internal/converter/converter.go`
- `internal/converter/converter_test.go`

**Estimated scope:** Small (2 files)

---

## Milestone 3 — Transporte HTTP

## Task 4: Router chi + handler `POST /convert` + `/health`

**Description:** Montar el router `chi/v5` y el handler que traduce el JSON al núcleo y mapea errores a RFC 9457.

**Acceptance criteria:**
- [ ] Router chi con `POST /convert` y `GET /health` (health → `200 {"status":"ok"}`)
- [ ] Decodifica `ConvertRequest`; `content` vacío → `400`; body inválido → `400`
- [ ] Entrada que supera `MAX_FILE_SIZE_MB` → `413`
- [ ] Mapeo de errores: `ErrInvalidMarkdown` → `422 "Invalid Markdown"`; `ErrConversion` → `500 "Conversion failed"`
- [ ] Respuesta 200: `content_base64` (base64) + `mime_type: application/pdf`
- [ ] Respuestas de error en RFC 9457 (`application/problem+json`)

**Verification:**
- [ ] `go test ./internal/httpserver/...` con `httptest` (200/400/413/422/500)

**Dependencies:** Task 2, Task 3

**Files likely touched:**
- `internal/httpserver/server.go`
- `internal/httpserver/handler_convert.go`
- `internal/httpserver/handler_health.go`
- `internal/httpserver/server_test.go`

**Estimated scope:** Medium (4 files)

---

## Milestone 4 — Integración + wiring + deploy

## Task 5: Tests de integración en `/test`

**Description:** Pruebas end-to-end en `/test` que ejercitan Pandoc+Typst sobre Markdown con estructuras complejas, para validar que el motor preserva tablas/headers y que el output es un PDF válido.

**Acceptance criteria:**
- [ ] Markdown con tablas, headers y listas → PDF con magic `%PDF` válido
- [ ] Verificación de que la estructura se preserva (sin pérdida de formato)
- [ ] Verificación de que no se crean archivos temporales (TMPDIR controlado)
- [ ] Los tests se omiten (`t.Skip`) si `pandoc`/`typst` no están en PATH

**Verification:**
- [ ] `go test ./test/...` con `pandoc`+`typst` instalados

**Dependencies:** Task 3

**Files likely touched:**
- `test/convert_integration_test.go`
- `test/testdata/` (fixtures Markdown)

**Estimated scope:** Small (2 files)

---

## Task 6: `main.go` (wiring + graceful shutdown)

**Description:** Ensamblar config, núcleo y server HTTP; graceful shutdown por señales.

**Acceptance criteria:**
- [ ] `cmd/pdf-converter/main.go` arma el grafo (config → converter → server) y arranca
- [ ] Señales (SIGINT/SIGTERM) → shutdown limpio (context cancel + `srv.Shutdown`)

**Verification:**
- [ ] `go build ./cmd/pdf-converter` y `go run` arranca sin panics

**Dependencies:** Task 4

**Files likely touched:**
- `cmd/pdf-converter/main.go`

**Estimated scope:** Small (1 file)

---

## Task 7: `Dockerfile` + `docker-compose.yml`

**Description:** Imagen multi-stage (Go estático + `pandoc` + `typst`) y el compose de servicio interno sin puertos expuestos.

**Acceptance criteria:**
- [ ] `Dockerfile` multi-stage: build Go (`CGO_ENABLED=0`) → imagen final con `pandoc` + `typst` + `ca-certificates`, usuario non-root, `EXPOSE 8000`, healthcheck
- [ ] `docker-compose.yml` con `MAX_FILE_SIZE_MB=${MAX_FILE_SIZE_MB:-20}`, `networks: [fast_pdf_network]`, **sin** `ports` al host
- [ ] El servicio arranca aislado (sin el `.env` raíz) con el default 20

**Verification:**
- [ ] `docker build .` compila la imagen
- [ ] `docker compose config` valida el compose

**Dependencies:** Task 6

**Files likely touched:**
- `Dockerfile`
- `docker-compose.yml`

**Estimated scope:** Small (2 files)

---

## Checkpoints

### Checkpoint: Milestone 1 — Foundation (tras Tasks 1-2)
- [ ] `go build ./...` y `go vet ./...` limpios
- [ ] DTOs y problem alineados con el contrato (`content` / `content_base64` + `mime_type`)

### Checkpoint: Milestone 2 — Núcleo (tras Task 3)
- [ ] `Convert` produce un PDF válido por pipes y no toca disco (TMPDIR vacío)

### Checkpoint: Milestone 3 — Transporte (tras Task 4)
- [ ] `POST /convert` end-to-end: 200 / 400 / 413 / 422 / 500 verificados con `httptest`

### Checkpoint: Milestone 4 — Complete (tras Tasks 5-7)
- [ ] `gofmt -l .` vacío; `go test ./...` y `go vet ./...` pasan
- [ ] Todos los success criteria del spec cubiertos
- [ ] Listo para REVIEW
