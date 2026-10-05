# Spec: `pdf-converter`

## 1. Resumen y Propósito
`pdf-converter` es un microservicio con una única responsabilidad: **convertir contenido en formato Markdown a un archivo PDF**, operando exclusivamente al momento de la descarga solicitada por el usuario (ADR-0005).

- **Tecnología:** Desarrollado en **Go** (Golang), garantizando alta concurrencia y bajo consumo de recursos en memoria.
- **Enrutamiento / Gateway:** Integrado con **Traefik** como reverse proxy / API gateway de la organización para el enrutamiento de peticiones.
- **Qué hace:** Expone un endpoint HTTP síncrono que recibe un payload con texto Markdown y devuelve un documento PDF binario listo para descarga.
- **Qué NO hace:** Ya no maneja la ingesta de archivos ni conversiones de subida (la ingesta de Markdown es directa hacia persistencia y la extracción cruda se realiza en `pdf-extractor`). No toma decisiones de negocio sobre cuándo convertir; solo ejecuta la conversión solicitada (ADR-0005, Plan).

---

## 2. Alcance y Arquitectura (Contexto en la Organización)
El servicio se integra dentro del ecosistema de microservicios de la organización (`pdf-main`, `pdf-extractor`, `pdf-persistance`, `pdf-validator`, etc.):
- **`pdf-main`**: Es el orquestador principal. Cuando un cliente solicita descargar un documento almacenado en formato canónico (Markdown), `pdf-main` invoca de forma síncrona a `pdf-converter`.
- **Independencia de Lógica y Transporte**: Siguiendo ADR-0004, la lógica núcleo de conversión está desacoplada del adaptador HTTP.

---

## 3. Especificación de Endpoints (Contrato HTTP)

### POST `/convert`
Convierte Markdown a PDF de forma síncrona.

- **Método:** `POST`
- **Content-Type:** `application/json`
- **Request Body (Ejemplo DTO):**
```json
{
  "markdown_content": "# Informe Técnico\n\nEste es el contenido en **Markdown** canónico.",
  "filename": "informe.pdf"
}
```
- **Response (Éxito - 200 OK):**
  - **Content-Type:** `application/pdf`
  - **Body:** Binario del PDF resultante.

- **Response (Error - RFC 9457 / ADR-0001):**
  - **Content-Type:** `application/problem+json`
  - **Ejemplo:**
```json
{
  "type": "about:blank",
  "title": "Error de conversión",
  "status": 422,
  "detail": "Fallo en la renderización de Pandoc: sintaxis LaTeX inválida",
  "instance": "/convert"
}
```

---

## 4. Restricciones Técnicas y Decisiones de Diseño (ADRs)

1. **Restricción de Memoria (RAM-only):** 
   - Está estrictamente prohibido el uso de archivos temporales en disco para procesar la conversión.
   - La ejecución de Pandoc se realiza mediante `subprocess` utilizando streams en memoria (`stdin`/`stdout` pipes).
2. **Motor de Renderizado:**
   - Pandoc empaquetado en el contenedor Docker junto con un motor de PDF eficiente (LaTeX o Weasyprint).
3. **Manejo de Errores:**
   - Todos los errores devuelven el formato estándar **RFC 9457** (`application/problem+json`) con un `title` estable (ADR-0001).

---

## 5. Criterios de Verificación (Testing y Pruebas de Estres)
- Pruebas unitarias sobre la función núcleo de conversión en Go.
- Verificación de renderizado correcto de estructuras complejas (tablas, encabezados, listas) sin pérdida de formato.
- Validación de manejo de errores ante entradas malformadas respetando RFC 9457.
- **Prueba de Estres (Stress Test):** Evaluación de rendimiento y concurrencia bajo carga elevada para medir latencia y uso de recursos (RAM/CPU) en el procesamiento con Pandoc a través de Traefik.
