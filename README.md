# PDF Converter

Microservicio interno en Go que convierte **Markdown a PDF** bajo demanda, como
wrapper de [Pandoc](https://pandoc.org/) y [Typst](https://typst.app/). Es el
servicio inverso de `pdf-extractor`: se invoca únicamente cuando el usuario
descarga un documento, para materializar el Markdown almacenado como PDF.

- **Qué hace:** expone `POST /convert` (síncrono). Recibe Markdown y devuelve el
  PDF codificado en base64 dentro de un JSON.
- **Qué no hace:** no ingesta, no persiste y no decide *cuándo* convertir. Solo
  `pdf-main` lo invoca (ADR-0005).
- **Restricción clave:** conversión **RAM-only**. No se escriben archivos
  temporales en disco en ningún punto del pipeline (ADR-0002).

## Pipeline de dos etapas

La vía directa de Pandoc, `pandoc -t pdf --pdf-engine=typst -o -`, **no es
RAM-only**: Pandoc crea `toPdfViaTempFile<N>-0.html` y `toPdfViaTempFile<N>-1.pdf`
en el directorio de trabajo durante la conversión. Eso incumple ADR-0002 y
además falla en el contenedor (`pandoc: .: openTempFile: permission denied`)
porque el servicio corre como usuario sin privilegios en un directorio no
escribible.

Por eso la conversión se implementa como un pipeline de dos procesos unidos por
tuberías:

```
 Markdown (stdin)
        │
        ▼
 ┌──────────────┐   Typst source   ┌────────────────────────────┐   PDF
 │   pandoc     │ ───────────────► │  typst compile --format pdf │ ─────► stdout
 │ -t typst -o -│     (stdout)     │  - /dev/stdout             │
 └──────────────┘                  └────────────────────────────┘
```

- **Etapa 1:** `pandoc -f markdown -t typst -o -` traduce Markdown a Typst y lo
  emite por stdout. Aquí se detecta el Markdown inválido (exit code `64`).
- **Etapa 2:** `typst compile --format pdf - /dev/stdout` compila el Typst leído
  de stdin y escribe el PDF directamente en stdout.

Detalles que importan:

- `typst compile --format pdf - -` **no** sirve: Typst interpreta el segundo `-`
  como nombre de archivo y crea un fichero literal llamado `-`. Hay que apuntar
  a `/dev/stdout`.
- `--format pdf` es obligatorio: Typst no puede inferir el formato de
  `/dev/stdout` por la extensión.
- El test `TestConvert_StraceNoDiskWrites` verifica con `strace` que no existan
  `open(..., O_CREAT|O_WRONLY|O_RDWR)` fuera de stdout durante la conversión.

## Características

- **RAM-only**: todo el flujo ocurre en memoria a través de pipes.
- **Núcleo desacoplado del transporte** (ADR-0004): `internal/converter` expone
  `Convert(ctx, timeout, markdown)` sin conocer DTOs ni HTTP.
- **Errores RFC 9457** (`application/problem+json`) en todas las respuestas de
  error.
- **Logging estructurado** (ADR-0007): JSON de una línea a `stdout` con
  `log/slog`.
- **Timeout configurable** por conversión (`CONVERT_TIMEOUT`) y límite de
  entrada (`MAX_FILE_SIZE_MB`).
- **Contenedor seguro**: binario estático (`CGO_ENABLED=0`), imagen multi-stage y
  usuario non-root (uid `10001`).

## Endpoints

### `POST /convert`

Convierte un documento Markdown a PDF.

**Request** — `Content-Type: application/json`:

```json
{
  "content": "# Título\n\n- Ítem uno\n- Ítem dos\n\n| A | B |\n|---|---|\n| 1 | 2 |\n"
}
```

**Respuesta (`200 OK`):**

```json
{
  "content_base64": "JVBERi0xLjQKJcOkw7zDtsOfCjIgMCBvYmoK...",
  "mime_type": "application/pdf"
}
```

`content_base64` es el PDF binario codificado en base64 (unos 18–25 KB para un
documento pequeño).

**Errores** (RFC 9457):

| Status | `title` | Causa |
|---|---|---|
| `400` | `Invalid Request` | JSON malformado, campo `content` vacío o campos desconocidos |
| `413` | `Payload Too Large` | Cuerpo o contenido supera `MAX_FILE_SIZE_MB` |
| `415` | `Unsupported Media Type` | `Content-Type` distinto de `application/json` |
| `422` | `Invalid Markdown` | Pandoc no puede parsear el Markdown (exit code `64`) |
| `500` | `Conversion failed` | Falla el motor de conversión o expira el timeout |

```json
{
  "type": "about:blank",
  "title": "Invalid Markdown",
  "status": 422,
  "detail": "The provided Markdown could not be converted",
  "instance": "/convert"
}
```

### `GET /health`

Endpoint trivial usado por el `HEALTHCHECK` de Docker:

```json
{ "status": "ok" }
```

## Variables de Entorno

| Variable | Descripción | Default |
|---|---|---|
| `HTTP_ADDR` | Dirección de escucha interna (no expuesta al host) | `:8000` |
| `LOG_LEVEL` | Nivel mínimo de logging (`debug`, `info`, `warn`, `error`) | `info` |
| `CONVERT_TIMEOUT` | Timeout por conversión (formato `time.Duration`) | `10s` |
| `MAX_FILE_SIZE_MB` | Tamaño máximo del Markdown de entrada en MB (ADR-0008: 1 MB = 1 048 576 bytes) | `20` |
| `PDF_CONVERTER_REPLICAS` | Réplicas a instanciar en Docker Compose | `1` |

## Logging

Los eventos se emiten a `stdout` en **JSON de una sola línea** (ADR-0007):

```json
{"time":"2026-11-10T14:29:09Z","level":"INFO","msg":"http request completed","service":"pdf-converter","http_method":"POST","http_path":"/convert","http_status":200,"bytes":24236,"duration_ms":103,"request_id":"7a1404959ccb/IrpefwHz9r-000002"}
```

- Campos base: `time` (UTC), `level`, `msg`, `service=pdf-converter`.
- Contexto HTTP: `http_method`, `http_path`, `http_status`, `bytes`,
  `duration_ms`, `request_id`.
- **Privacidad estricta:** nunca se registra el Markdown de entrada ni el PDF de
  salida, ni siquiera en nivel `debug`.

## Desarrollo local

Requiere **Go 1.26+** y, para los tests de integración, `pandoc` y `typst` en el
`PATH`.

```bash
# Compilar y ejecutar
go build ./...
go run ./cmd/pdf-converter

# Suite de tests (los de integración se saltan si faltan pandoc/typst)
go test ./...

# Formato y vet
gofmt -l .
go vet ./...
```

Para ejecutar la suite de integración completa (incluye `strace` y validación de
texto del PDF) hace falta además `poppler-utils`:

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.26-alpine sh -c '
  apk add --no-cache pandoc typst poppler-utils strace
  CGO_ENABLED=0 go test -count=1 ./...'
```

## Docker

El `Dockerfile` es multi-stage:

1. **build**: `golang:1.26-alpine`, compila un binario estático
   (`CGO_ENABLED=0`, `-trimpath -ldflags="-s -w"`).
2. **runtime**: `alpine:3.20` con `ca-certificates`, `pandoc` y `typst`, y usuario
   `app` (uid `10001`).

```bash
# Construir y levantar
docker compose up -d --build

# Escalar horizontalmente
PDF_CONVERTER_REPLICAS=5 docker compose up -d
```

> El servicio no expone puertos al host: vive en la red interna
> `fast_pdf_network` y `pdf-main` lo alcanza por DNS interno en
> `http://pdf-converter:8000`.
>
> **Nota de despliegue:** por ahora el `docker-compose.yml` raíz del monorepo no
> incluye este servicio; hay que añadirlo al `include` para que arranque con el
> stack completo.

## Documentación

- [`SPEC.md`](SPEC.md): especificación funcional y técnica.
- [`tasks/plan.md`](tasks/plan.md): plan de implementación y decisiones de
  arquitectura.
- [`tasks/todo.md`](tasks/todo.md): tareas, criterios de aceptación y
  verificación.
- [`docs/plan-pdf-converter.md`](docs/plan-pdf-converter.md): contexto de
  producto dentro del monorepo.
