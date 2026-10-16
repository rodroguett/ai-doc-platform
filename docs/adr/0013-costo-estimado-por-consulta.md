# ADR-0013: Registrar cada consulta con su costo estimado a tarifa de lista

- **Estado:** aceptado
- **Fecha:** 2026-10-15
- **Issue relacionado:** #65

## Contexto

El proyecto promete control explícito del costo por consulta, pero hoy no
lo mide. La traza de cada respuesta tiene un campo `CostUSD` que nunca se
asigna, el tope diario de 500 consultas nuevas se cuenta en memoria y se
pierde cuando Cloud Run apaga la instancia, y `GET /v1/usage` responde 501
aunque el contrato ya define su respuesta.

Las fuerzas relevantes:

- **El costo real es cero.** Embeddings y generación corren en la capa
  gratuita de Gemini, con una clave en un proyecto sin facturación
  ([ADR-0008](0008-embeddings-gemini.md),
  [ADR-0012](0012-generacion-gemini-flash-lite.md)). Un registro que solo
  diga "cero" no informa nada.
- **La pregunta que importa es cuánto costaría.** Si la demo pasara a la
  capa de pago, o si el sistema se usara de verdad, el costo dependería del
  volumen y del tamaño de cada consulta. Eso es lo que hay que saber antes
  de decidir.
- **Las preguntas pueden ser sensibles.** La demo es pública y avisa que
  las preguntas se envían a Google (#56). No hay motivo para guardarlas
  además.
- **La trazabilidad es la restricción que ordena la arquitectura.** Cada
  respuesta tiene un identificador de traza; el costo de una consulta
  debería poder seguirse desde ese identificador.
- **Las tarifas cambian**, y los modelos se retiran: `gemini-embedding-001`
  tiene fecha de apagado el 14 de mayo de 2028 y `gemini-3.1-flash-lite`,
  el 7 de mayo de 2027.

Lo medido el 2026-10-15, con el prompt de sistema vigente y cinco pasajes
del corpus: una consulta típica tiene unos 3.600 tokens de entrada y unos
60 de salida. Ninguno de los dos modelos de generación produjo tokens de
razonamiento con la configuración actual.

## Decisión

Guardaremos una fila por consulta con su consumo de tokens y un costo
estimado con la tarifa de lista de la capa de pago, sin el texto de la
pregunta ni de la respuesta.

**Qué se guarda.** Por cada consulta atendida: fecha y hora, identificador
de traza, colección, proveedor y modelo que redactó, tokens de entrada y de
salida de la generación, si vino del caché, cómo terminó (respondida,
degradada porque el modelo falló, degradada porque los pasajes no
respondían, sin pasajes) y el costo estimado. No se guardan la pregunta, la
respuesta, los pasajes ni la dirección de origen.

**Costo estimado, no real.** El costo se calcula con la tarifa de lista de
la capa de pago de cada modelo y se rotula como estimado en todas partes:
en la columna, en la traza y en `GET /v1/usage`. El costo real, cero, no se
registra: lo garantiza la configuración, no la medición.

**El costo se fija al registrar.** Cada fila guarda el costo calculado con
las tarifas vigentes en ese momento. Si la tarifa cambia, el historial no se
recalcula: muestra lo que habría costado cuando ocurrió.

**Las tarifas viven en el código**, en el adaptador de cada proveedor, con
la fecha de consulta y la fuente. Las vigentes, tomadas de
<https://ai.google.dev/gemini-api/docs/pricing> el 2026-10-15, por millón
de tokens:

| Modelo | Entrada | Salida |
|---|---|---|
| `gemini-3.5-flash-lite` | 0,30 USD | 2,50 USD |
| `gemini-3.1-flash-lite` | 0,25 USD | 1,50 USD |
| `gemini-embedding-001` | 0,15 USD | — |

La de `gemini-embedding-001` ya no aparece en la página de precios, que
lista su reemplazo; se toma del anuncio de disponibilidad general de
Google, del 14 de julio de 2025. Actualizar una tarifa es un pull request.
Un modelo sin tarifa conocida registra el costo como desconocido, nunca
como cero, y lo advierte en el log; un test verifica que los modelos
configurados tengan tarifa.

**Qué se cuenta.**

- *Generación:* los tokens que informa el proveedor. La tarifa de salida
  incluye los de razonamiento: si el proveedor los informa aparte, se suman
  a la salida. Hoy el cliente lee solo `candidatesTokenCount`, y debe
  leerlos también.
- *Respaldo:* si el primer modelo falla y redacta el segundo, se registra
  el segundo. Un intento rechazado con 429 o 503 no se factura; uno que
  se corta por tiempo de espera podría facturarse, y no se cuenta.
- *Acierto de caché:* se registra con cero tokens y costo cero. Cuenta para
  la tasa de aciertos y no para el tope diario.
- *Embedding de la pregunta:* no se cuenta. Son unas decenas de tokens,
  menos del 0,5 % del costo de una consulta, y el proveedor no informa
  cuántos fueron.
- *Ingesta:* no se registra por consulta. Embeber la RCA de prueba son unos
  112.000 tokens, unos 0,017 USD a tarifa de lista, una vez por documento.

**El tope diario se calcula desde este registro** (#68): las consultas del
día que no vinieron del caché.

## Alternativas consideradas

### No persistir nada

Mantener el conteo en memoria, como hoy. Es lo más simple y lo más privado.
Se descartó porque es justamente lo que no funciona: Cloud Run apaga la
instancia sin tráfico, y con ella se pierden el conteo y el tope. Un
control de costo que se reinicia solo no controla nada.

### Agregados diarios en vez de una fila por consulta

Una fila por día y modelo con las sumas. Ocupa menos y guarda todavía
menos. Se descartó porque rompe la trazabilidad: no permite responder
cuánto costó una respuesta concreta a partir de su identificador de traza.
A la escala del tope diario, una fila por consulta son a lo más 500 filas
de unos cien bytes por día.

### Métricas basadas en los logs de Cloud Logging

Cada consulta ya deja un log; Cloud Logging puede derivar métricas de él.
Se descartó porque ata `GET /v1/usage` a la API de Google Cloud, deja el
endpoint inútil fuera de Cloud Run, y los logs se retienen 30 días por
omisión.

### Registrar solo el costo real

Guardar cero, que es lo que se paga. Se descartó porque no responde la
pregunta que motiva el registro: cuánto costaría.

## Consecuencias

### Positivas

- El costo de una respuesta se sigue desde su identificador de traza.
- El tope diario sobrevive a los reinicios.
- Se puede responder con datos cuánto costaría la demo en la capa de pago.
  Con lo medido, una consulta son unos 0,0012 USD; el tope de 500 consultas
  diarias, unos 0,62 USD por día, o unos 19 USD al mes si se alcanzara
  todos los días. Eso supera el presupuesto del proyecto: pasar a la capa
  de pago exigiría bajar el tope.

### Negativas

- **Las tarifas se mantienen a mano.** Si Google las cambia, el costo
  estimado queda desactualizado hasta que alguien lo note. Nada lo avisa.
- **El costo estimado parece un costo.** Aunque se rotule, un lector
  apurado puede tomarlo por lo que se paga. Por eso el rótulo va en cada
  lugar donde aparece.
- **El costo no es completo.** Omite el embedding de cada pregunta y la
  ingesta, y no incluye Cloud Run ni Neon. Es el costo del proveedor de
  modelos por consulta, no el costo de operar el sistema.
- **Una escritura más por consulta** en Neon. Si falla, la consulta se
  responde igual y el registro se pierde: se prefiere responder a contar.
  El tope diario puede quedar corto por esas pérdidas.
- **Se guarda la colección consultada**, que dice algo del interés de quien
  pregunta. Se acepta porque las colecciones son públicas y pocas.
- **La tabla crece sin límite.** A lo más 500 filas diarias, unos 18 MB al
  año, dentro de los 512 MB de la capa gratuita de Neon. No se define
  retención todavía.

### Neutras

- `gemini-embedding-001` se apaga el 14 de mayo de 2028 y
  `gemini-3.1-flash-lite` el 7 de mayo de 2027. Cambiar de modelo exige
  agregar su tarifa, y en el caso de los embeddings, volver a ingerir.

## Revisión

Reconsiderar si el proyecto pasa a la capa de pago: entonces el costo real
deja de ser cero y conviene contrastarlo con la facturación. También si la
tabla se acerca al límite de almacenamiento de Neon, o si aparece un
segundo proveedor cuyas tarifas no se expresen por token.
