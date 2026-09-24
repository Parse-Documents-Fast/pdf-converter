# plan.md — pdf-converter

## Qué hay que construir
Un wrapper de Pandoc con una sola responsabilidad (ADR-0005): convertir Markdown a PDF, únicamente al momento de la descarga. Ya no tiene responsabilidad de ingesta — un Markdown subido se persiste tal cual, sin pasar por este servicio.

## Cómo construirlo, en orden
1. Confirmar que Pandoc está disponible en el contenedor (instalarlo en el Dockerfile) junto con un motor de renderizado para PDF (LaTeX o `weasyprint`, decidir cuál según qué tan pesada quede la imagen).
2. Implementar la llamada por subprocess a Pandoc usando stdin/stdout (pipes), nunca archivos temporales en disco — por la regla de RAM.
3. Un único endpoint: recibe Markdown, devuelve PDF.
4. Probar con contenido que tenga tablas y headers, para confirmar que Pandoc no pierde esa estructura al convertir.

## Transporte
Siempre HTTP síncrono — el cliente está esperando activamente el archivo en esa misma conexión, no hay caso de uso asíncrono para este servicio (a diferencia de lo que se había planteado antes de ADR-0005, ya no tiene un camino de ingesta que justifique cola).

## Fuera de alcance
Cualquier lógica de decisión sobre cuándo convertir — eso lo decide `pdf-main`, este servicio solo ejecuta la conversión que le piden.
