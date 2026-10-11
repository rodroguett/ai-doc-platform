# ADR-0008: Generar los embeddings con Gemini en su capa gratuita

- **Estado:** aceptado
- **Fecha:** 2026-10-11
- **Issue relacionado:** #32

## Contexto

La búsqueda semántica necesita convertir cada fragmento de un documento, y
cada consulta, en un vector. El modelo que lo hace fija dos cosas difíciles
de cambiar después: la dimensión de la columna de vectores en el esquema, y
el espacio en que viven todos los vectores ya calculados. Cambiar de modelo
obliga a reprocesar el corpus completo.

Las fuerzas relevantes:

- **El demo no puede generar un cobro.** El tope es un dólar, y la forma
  más segura de respetarlo es que no exista un medio de pago.
- El corpus es pequeño. Una RCA de trescientas páginas son unos 250.000
  tokens; una consulta, unos 30. A los precios de cualquier proveedor,
  ingerir una RCA cuesta entre medio centavo y cuatro centavos de dólar. El
  precio por token no distingue a las alternativas.
- Lo que sí las distingue son los límites de uso: la demo es pública, y
  cada consulta necesita una petición de embeddings.
- Los documentos están en español y son texto legal y técnico.
- El servicio corre con 256 MiB de memoria y escala a cero: no hay dónde
  alojar un modelo propio.
- Las RCA, sus adendas y los informes de seguimiento son documentos
  públicos.

## Decisión

Usaremos `gemini-embedding-001`, de la API de Gemini de Google, en su capa
gratuita, con vectores de 768 dimensiones.

**Capa gratuita, sin facturación.** La clave se crea en un proyecto de
Google distinto del que aloja el servicio, y sin cuenta de facturación
asociada. Mientras el proyecto no tenga facturación, la API no puede
cobrar: al agotar la cuota responde con un error, no con un cargo.

**768 dimensiones.** El modelo produce 3.072 y permite truncar el vector a
1.536 o 768 conservando la mayor parte de la calidad; según la
documentación de Google, la diferencia entre 3.072 y 768 es de décimas en
su benchmark. El índice HNSW de pgvector admite hasta 2.000 dimensiones, de
modo que 3.072 no es indexable sin cambiar de tipo de columna. Entre las dos
que caben, 768 ocupa la mitad y es más rápida de comparar. Los vectores
truncados deben normalizarse antes de guardarse.

**El tipo de tarea se declara.** Los fragmentos se generan como
`RETRIEVAL_DOCUMENT` y las consultas como `RETRIEVAL_QUERY`, que es para lo
que el modelo distingue ambos casos.

**El proveedor queda detrás de una interfaz en `llmgw`**, y el nombre del
modelo y la dimensión se guardan junto a cada fragmento. Así un cambio de
proveedor es configuración más reprocesar, y dos generaciones de vectores
no se mezclan por error en una misma búsqueda.

### Costo

| | Tokens | Capa gratuita | Si fuera de pago (0,15 USD por millón) |
|---|---|---|---|
| Ingerir una RCA de 300 páginas | ~250.000 | 0 USD | ~0,04 USD |
| Una consulta | ~30 | 0 USD | ~0,000005 USD |

El costo mensual previsto es cero.

### Límites que impone la capa gratuita

Según el foro oficial de Google, a la fecha de este ADR:

| Límite | Valor | Consecuencia |
|---|---|---|
| Tokens por minuto | 30.000 | Ingerir una RCA toma unos nueve minutos |
| Peticiones por minuto | 100 | Holgado para consultas |
| Peticiones por día | 1.000 | Techo diario de consultas más ingesta |
| Tokens por texto | 2.048 | Ningún fragmento puede superarlo |

### Verificado con la cuenta real

Con la clave ya creada, el 2026-10-11:

- El modelo responde en la capa gratuita, y el proyecto que aloja la clave
  no tiene facturación asociada.
- Con `outputDimensionality: 768` devuelve 768 valores con norma 0,58: la
  normalización posterior es necesaria. Sin ese parámetro devuelve 3.072
  valores ya normalizados.
- Una petición tarda alrededor de un segundo.
- Un fragmento sobre monitoreo de aguas y una consulta sobre lo mismo
  obtienen similitud coseno 0,70. Es una sola muestra: confirma que el
  modelo funciona, no mide su calidad.
- **Dos de unas quince peticiones fallaron con `403 PERMISSION_DENIED`** y
  la siguiente, idéntica, respondió bien. No es el error de cuota, que
  sería 429. La clave tenía minutos de creada, lo que sugiere propagación
  pendiente, pero no está confirmado.
- La respuesta no trae cabeceras con la cuota restante: los límites no
  pueden leerse desde la API.

## Alternativas consideradas

### Voyage AI, sin medio de pago

Ofrece 200 millones de tokens gratuitos, calidad comparable y un modelo
específico para texto legal. Es el proveedor de embeddings que recomienda
Anthropic.

Se descartó por su límite sin tarjeta: tres peticiones por minuto y 10.000
tokens por minuto. La demo pública admitiría tres consultas por minuto, y
ninguna mientras se ingiere un documento. Con tarjeta los límites suben,
pero aparece un medio de pago, y no se pudo confirmar que permita fijar un
tope de gasto.

Es la alternativa a reconsiderar primero.

### OpenAI `text-embedding-3-small`, con saldo prepagado

Es el esquema que el proyecto había previsto: saldo prepagado sin recarga
automática, que acota el gasto al saldo. Los límites son altos y una RCA se
ingiere en menos de un minuto.

Se descartó porque la compra mínima de saldo es de cinco dólares, por
encima del tope fijado para el demo, y porque en los benchmarks
multilingües consultados queda por debajo de las otras dos.

### Un modelo abierto alojado en el propio servicio

Sin costo por token ni dependencia de un tercero, y sin enviar los
documentos a nadie.

Se descartó porque un modelo multilingüe de calidad comparable necesita del
orden de uno o dos gigabytes de memoria, varias veces lo que el servicio
tiene asignado, y una instancia de ese tamaño que no escale a cero supera
el presupuesto.

### `gemini-embedding-2`

Es el modelo más reciente del mismo proveedor, multimodal. Se descartó
porque el corpus es solo texto, su precio de lista es mayor, y no se pudo
confirmar que tenga capa gratuita.

## Consecuencias

### Positivas

- El demo no tiene costo ni puede generarlo.
- Un solo proveedor puede cubrir también la generación de respuestas, con
  lo que el sistema entero dependería de una sola cuenta sin medio de pago.
  Esa decisión se toma en #25.
- Es el modelo con mejor puntaje multilingüe entre los considerados.

### Negativas

- **Google usa lo que se envía a la capa gratuita para mejorar sus
  productos, y personas pueden revisarlo.** Es aceptable porque los
  documentos del demo son públicos. Un cliente con documentos internos no
  podría usar esta configuración: tendría que pasar al nivel de pago, que
  no tiene ese uso, o a otro proveedor.
- **La cuota gratuita puede cambiar sin aviso.** En el foro de Google hay
  reportes de recortes. Si se reduce o desaparece, la demo deja de ingerir
  y de responder hasta cambiar de nivel o de proveedor.
- **Mil peticiones diarias es el techo de la demo**, compartido entre
  consultas e ingesta. Nada lo reparte ni avisa cuando se acerca.
- **La ingesta es lenta**: unos nueve minutos por RCA, limitada por los
  tokens por minuto. Agrava el problema, ya registrado en
  [ADR-0005](0005-monolito-modular.md), de ejecutar trabajos largos en
  Cloud Run.
- **La seguridad contra cobros depende de un detalle de configuración.**
  Si la clave se crea en un proyecto con facturación, o alguien se la
  asocia después, la misma clave pasa al nivel de pago sin que el código
  lo note.
- **Cambiar de modelo obliga a reprocesar todo el corpus**, y durante el
  cambio conviven vectores incomparables entre sí.
- **Ningún fragmento puede superar los 2.048 tokens.** Condiciona la
  fragmentación (#33).
- **La calidad no está medida sobre este corpus.** Los puntajes provienen
  de benchmarks generales publicados por terceros, no de una evaluación
  sobre texto legal chileno. Medirla requiere el conjunto de preguntas de
  evaluación, que aún no existe.
- **Los límites citados provienen del foro de Google, no de su tabla
  oficial de cuotas**, y la API no los informa. Se conocerán con certeza
  al alcanzarlos.
- **La API falla de forma intermitente con un error que no es de cuota.**
  La ingesta debe reintentar ante cualquier fallo transitorio, no solo
  ante un 429, y una consulta puede fallar sin causa atribuible al
  usuario.
- **No hay forma de saber cuánta cuota queda.** El conteo de uso debe
  llevarlo el propio sistema.

### Neutras

- Truncar a 768 dimensiones es una decisión reversible solo reprocesando:
  pasar a 1.536 exige una migración de la columna y recalcular los
  vectores.
- La clave de la API es un secreto más en Secret Manager, el tercero.

## Revisión

Reconsiderar si la cuota gratuita deja de alcanzar para la demo, si cambian
sus condiciones de uso de datos, o si el corpus pasa a incluir documentos
no públicos. La alternativa a evaluar primero es Voyage AI.

Reconsiderar la dimensión si una evaluación sobre el corpus muestra que 768
pierde recuperación relevante frente a 1.536.
