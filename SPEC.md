# Spec: pdf-converter

## Objective

`pdf-converter` es un microservicio con una única responsabilidad (ADR-0005):
convertir contenido **Markdown a PDF** únicamente al momento de la descarga. Es
un wrapper de **Pandoc**: recibe Markdown, ejecuta Pandoc, devuelve el PDF.

- **Qué hace:** expone `POST /convert`, síncrono. Recibe Markdown y devuelve el
  PDF binario codificado en base64 dentro de JSON.
- **Qué NO hace:** no hace ingesta (un Markdown subido se persiste tal cual), no
  decide *cuándo* convertir — `pdf-main` lo invoca y él solo ejecuta la
  conversión solicitada (ADR-0005).
- **Consumidor:** solo `pdf-main` (servicio interno, sin ruta pública).

## Tech Stack

- **Lenguaje:** Go `1.26.8` (módulo `github.com/Parse-Documents-Fast/pdf-converter`).
- **Router HTTP:** `chi/v5`.
- **Logging:** `log/slog`, JSON una línea a stdout (ADR-0007).
- **Motor de conversión:** Pandoc (`pandoc >= 3.1.8`) + **Typst**
  (`--pdf-engine=typst`). Render en memoria, sin archivos temporales.
- **Docker:** multi-stage (`CGO_ENABLED=0`, binario estático) + imagen final con
  `pandoc` + `typst` + `ca-certificates`, usuario non-root.

## Commands

```bash
# Build
go build ./...

# Test
go test ./...

# Vet / fmt
go vet ./...
gofmt -l .

# Run (requiere pandoc + typst en PATH)
go run ./cmd/pdf-converter

# Run en Docker
docker compose up --build -d
```

## Project Structure

```
cmd/pdf-converter/main.go          → entrypoint (config + server)
internal/config/                   → carga de env vars, defaults, Load() testeable
internal/dto/                      → ConvertRequest / ConvertResponse (wire JSON)
internal/problem/                  → RFC 9457 helpers (espejo de pdf-main)
internal/converter/                → núcleo puro: Markdown → PDF vía Pandoc
internal/httpserver/               → capa delgada: router chi, handlers
Dockerfile                         → multi-stage, pandoc + typst, non-root
docker-compose.yml                 → red fast_pdf_network, sin exponer puertos
tasks/plan.md, tasks/todo.md       → plan y tareas (fases siguientes del SDD)
```

## Config

Variables de entorno; defaults en código, sobrescribibles por el `.env` raíz vía
interpolación del compose:

| Variable | Default | Descripción |
|---|---|---|
| `HTTP_ADDR` | `:8000` | Dirección del servidor HTTP |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `CONVERT_TIMEOUT` | `10s` | Timeout del subproceso Pandoc |
| `MAX_FILE_SIZE_MB` | `20` | Límite de tamaño de la entrada Markdown (ADR-0008) |

- `MAX_FILE_SIZE_MB` sigue ADR-0008: 1 MB = 1 048 576 bytes → 20 MB =
  20 971 520 bytes. En el compose se inyecta desde el `.env` raíz:
  `MAX_FILE_SIZE_MB=${MAX_FILE_SIZE_MB:-20}`. El valor del default en código
  también es 20, para que un servicio levantado aislado se comporte igual.

## Code Style

Go idiomático, `gofmt`, doc comments en inglés (convención de los repos
hermanos). Núcleo separado del transporte (ADR-0004): la función de conversión
es pura y testeable; el handler es una capa delgada.

```go
// Convert ejecuta Pandoc leyendo de stdin y escribiendo a stdout (RAM-only).
// No crea archivos temporales.
func Convert(ctx context.Context, cfg Config, markdown string) ([]byte, error) {
    cmd := exec.CommandContext(ctx, "pandoc",
        "-f", "markdown",
        "-t", "pdf",
        "--pdf-engine", "typst",
        "-o", "-")
    cmd.Stdin = strings.NewReader(markdown)
    var out, stderr bytes.Buffer
    cmd.Stdout = &out
    cmd.Stderr = &stderr
    if err := cmd.Run(); err != nil {
        return nil, fmt.Errorf("pandoc: %w: %s", err, stderr.String())
    }
    return out.Bytes(), nil
}
```

Convenciones:
- DTOs con tags JSON `snake_case`; binario como `[]byte` (base64 en wire).
- Errores RFC 9457, `title` estable: `"Invalid Markdown"` (422, error de parse
  de Pandoc) y `"Conversion failed"` (fallo genérico del motor).
- Sin comentarios en español en el código; doc comments en inglés.

## Testing Strategy

- **Framework:** `testing` stdlib + `httptest` para los handlers.
- **Núcleo (unit):** `converter_test` — validar pipe stdin/stdout, propagación
  del `stderr` de Pandoc, timeout vía `context`, y estructura preservada
  (tablas, headers, listas) sobre un PDF de prueba generado por Pandoc real.
- **Handler (unit):** `httptest` sobre `chi` — 200 válido, 400 body inválido,
  405 método, 413 entrada sobre `MAX_FILE_SIZE_MB`, 422 Markdown inválido, y
  problema RFC 9457 en fallo de Pandoc.
- **Config (unit):** `Load()` con env y defaults.
- **Integración (`/test`, requiere `pandoc`+`typst`):** conversión end-to-end de
  un Markdown con tablas/headers y verificación de que el output es un PDF válido
  (magic `%PDF`). Las pruebas de renderizado de Typst viven en `/test`, como
  parte natural del desarrollo.
- **Cobertura:** núcleo y DTOs cubiertos; el loop del server queda delgado.

## Boundaries

- **Always:** validar inputs (content no vacío, tamaño límite), devolver RFC 9457,
  loggear sin contenido del documento, tests antes de cerrar, `gofmt`/`go vet`.
- **Ask first:** cambiar motor PDF (Typst↔WeasyPrint), añadir dependencias,
  modificar el contrato (campo nuevo o código de error), cambiar CI/Dockerfile.
- **Never:** escribir archivos temporales en disco (RAM-only), loguear el
  contenido del Markdown/PDF, exponer el puerto públicamente, usar un código de
  error distinto del acordado (422 para Markdown inválido) sin coordinar.

## Success Criteria

- `POST /convert` con `{"content":"# T\n\ntexto"}` devuelve 200 con
  `content_base64` decodificable a un PDF válido (`%PDF-`).
- El servicio no crea ningún archivo temporal durante la conversión (verificable
  en test inspeccionando que solo se usan pipes).
- Markdown inválido (parse error de Pandoc) devuelve `422` con
  `title: "Invalid Markdown"`; fallo genérico del motor devuelve `title:
  "Conversion failed"`. Ambos capturan el `stderr` de Pandoc.
- Entrada que supera `MAX_FILE_SIZE_MB` (20 MB por defecto) se rechaza con `413`.
- Imagen Docker final con `pandoc` + `typst` funcional y usuario non-root.
- `go build ./...`, `go vet ./...` y `go test ./...` limpios.

## Open Questions

- Ninguna pendiente. El 422 de "Markdown inválido" está acordado; queda una
  **issue en `pdf-main`** para que propague ese código en lugar de un `503`
  genérico. El límite de tamaño es `MAX_FILE_SIZE_MB` (ADR-0008, default 20 MB
  desde el `.env` raíz).
