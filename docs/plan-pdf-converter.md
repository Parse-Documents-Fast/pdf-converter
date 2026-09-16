# plan.md — pdf-converter

## Qué hay que construir
Un wrapper de Pandoc con dos usos distintos: convertir Markdown a HTML al momento de subir un archivo Markdown, y convertir HTML a PDF o a Markdown al momento de descargar un documento ya persistido.

## Cómo construirlo, en orden
1. Confirmar que Pandoc está disponible en el contenedor (instalarlo en el Dockerfile) junto con un motor de renderizado para HTML→PDF (LaTeX o `weasyprint`, decidir cuál según qué tan pesada quede la imagen).
2. Implementar la llamada por subprocess a Pandoc usando stdin/stdout (pipes), nunca archivos temporales en disco — por la regla de RAM.
3. Un único endpoint de ingestión: recibe Markdown, devuelve HTML.
4. Un único endpoint de salida: recibe HTML + formato pedido (`pdf` o `markdown`), devuelve el archivo convertido.
5. Probar la conversión de ida y vuelta con contenido que tenga tablas y headers, para confirmar que Pandoc no pierde esa estructura entre formatos.

## Fuera de alcance
Cualquier lógica de decisión sobre cuándo convertir — eso lo decide `pdf-main`, este servicio solo ejecuta la conversión que le piden.

## Actualización — transporte, distinto por responsabilidad (ADR-0004)
Este servicio tiene dos caminos con transporte diferente, no uno solo:

- **Ingesta (Markdown → HTML, al subir un archivo)**: deja de ser HTTP. Consume jobs de `queue:conversion` (Redis Streams, consumer group con `XACK`), y publica el resultado en `queue:conversion-results` — mismo patrón que `pdf-extractor`, porque es igual de terminal: nadie espera esa respuesta en el mismo request.
- **Descarga (HTML → formato pedido, al bajar un documento)**: sigue siendo HTTP síncrono. El cliente está esperando activamente el archivo en esa misma conexión — no hay "pending" posible acá.

La función núcleo de conversión (el wrapper de Pandoc en sí) no cambia entre los dos casos — solo cambia el adaptador que la invoca en cada camino (consumer de stream para ingesta, handler HTTP para descarga), según ADR-0004.
