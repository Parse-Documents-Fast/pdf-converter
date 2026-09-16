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
