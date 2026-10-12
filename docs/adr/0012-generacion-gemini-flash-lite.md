# ADR-0012: Redactar las respuestas con Gemini Flash-Lite en su capa gratuita

- **Estado:** aceptado
- **Fecha:** 2026-10-12
- **Issue relacionado:** #50

## Contexto

Las consultas ya recuperan los pasajes del corpus que sustentan una
respuesta ([ADR-0011](0011-recuperacion-y-modo-degradado.md)). Falta que un
modelo redacte esa respuesta a partir de ellos, marcando qué pasaje sustenta
cada afirmación.

Redactar una respuesta es enviarle al modelo la pregunta y unos cuatro
pasajes, y recibir un párrafo. Medido con preguntas reales sobre la RCA de
prueba, son entre 1.100 y 3.000 tokens de entrada y entre 16 y 150 de
salida.

Las fuerzas relevantes:

- **El demo no puede generar un cobro.** Es la misma restricción que decidió
  los embeddings ([ADR-0008](0008-embeddings-gemini.md)).
- La API es pública, y cada consulta nueva sería una petición de generación.
- Lo que importa en este dominio es la fidelidad: que el modelo no agregue
  nada que los pasajes no digan, y que cite.
- El sistema ya responde sin generación, en modo degradado. Un proveedor de
  generación que falla no deja a la consulta sin respuesta.
- Los documentos del demo son públicos.

## Decisión

Usaremos `gemini-3.5-flash-lite`, de la API de Gemini, en su capa gratuita y
con la misma clave que ya genera los embeddings. Si no responde, se
intentará con `gemini-3.1-flash-lite`; si tampoco, la consulta se responde
en modo degradado.

**La misma cuenta, sin facturación.** No se agrega proveedor, secreto ni
medio de pago. La cuota de generación es independiente de la de embeddings:
Google las limita por modelo.

**Un segundo modelo como respaldo, antes de degradar.** Los modelos de la
capa gratuita responden 503 por alta demanda con frecuencia, y no todos a
la vez. Intentar con otro cuesta una petición y evita degradar la respuesta.

**El proveedor queda detrás de una interfaz en `llmgw`**, como los
embeddings. La traza de cada respuesta declara qué modelo la redactó.

**Temperatura cero.** La misma pregunta sobre los mismos pasajes debe
producir, en lo posible, la misma respuesta.

### Mediciones

Con la clave del proyecto, el 2026-10-12. Cuatro preguntas sobre la RCA de
prueba, cada una con cuatro pasajes reales: tres que los pasajes responden y
una que no.

| Modelo | Respondió | Tiempo |
|---|---|---|
| `gemini-3.5-flash-lite` | 4 de 4, y 3 de 3 sondeos posteriores | 1,4 a 2,0 s |
| `gemini-3.1-flash-lite` | 2 de 4; 1 de 2 sondeos | 6 a 8 s |
| `gemini-3.5-flash` | 0 de 8 con preguntas reales; 2 de 2 con una petición trivial | — |
| `gemini-3.8-flash` | 0 de 6 | — |
| `gemini-2.5-flash`, `gemini-2.5-flash-lite` | Retirados para cuentas nuevas (404) | — |

Todas las fallas fueron `503 UNAVAILABLE`, "alta demanda"; ninguna fue de
cuota.

Sobre la calidad de `gemini-3.5-flash-lite`:

- Los valores que entregó coinciden con la fuente: los límites de 150 y
  50 µg/m³N de material particulado, la norma de referencia, los once años
  de vida útil.
- Todas las marcas de cita apuntaron a pasajes que existían.
- Ante la pregunta que los pasajes no responden, dijo que no contenían esa
  información y no citó ninguno.
- En una respuesta citó un pasaje de más: atribuyó la vida útil de once
  años también al pasaje que solo da las fechas de la fase de cierre.

### Costo

En la capa gratuita, cero. Como referencia de lo que costaría la
alternativa de pago más barata considerada, una consulta de 4.000 tokens de
entrada y 300 de salida con Claude Haiku 4.5 son unos 0,0055 dólares.

## Alternativas consideradas

### Un modelo Flash más capaz de la misma capa gratuita

`gemini-3.5-flash` y `gemini-3.8-flash` son modelos mayores, y en principio
más fieles. Se descartaron porque no responden: en la capa gratuita
devolvieron 503 en todas las peticiones de tamaño real. Un modelo mejor que
no contesta deja a la demo siempre en modo degradado.

### Groq, capa gratuita

No pide tarjeta y no usa los datos para entrenar. Se descartó por su límite
de 8.000 tokens por minuto en los modelos grandes: con consultas de 3.000 a
4.000 tokens, admite dos por minuto en toda la demo. Además agrega una
cuenta y un secreto.

### Claude Haiku 4.5, con saldo prepagado

Es la alternativa que se elegiría sin la restricción de costo: un modelo
más capaz, con un proveedor que no usa los datos para entrenar y que ofrece
citas verificables como función de la API. El saldo prepagado acota el
gasto al monto comprado.

Se descartó porque exige comprar saldo por adelantado, por encima del tope
fijado para el demo. Es la alternativa a reconsiderar primero.

### No generar: quedarse con los pasajes

El modo degradado ya responde con las citas. Se descartó como estado final
porque no responde la pregunta: deja el trabajo de leer y sintetizar a quien
consulta, que es lo que el sistema se propone ahorrar.

## Consecuencias

### Positivas

- El demo sigue sin costo y sin poder generarlo.
- No se agrega ninguna cuenta ni secreto.
- La generación tarda uno o dos segundos.
- El respaldo entre dos modelos existe desde el primer día, y el modo
  degradado queda como último recurso.

### Negativas

- **Es el modelo más pequeño de los disponibles.** Se eligió porque es el
  que responde, no porque sea el más fiel. Su calidad se evaluó con cuatro
  preguntas.
- **Citó un pasaje de más en una de cuatro respuestas.** Que una marca
  apunte a un pasaje existente se puede verificar; que ese pasaje sustente
  la afirmación, no. Una cita holgada pasa cualquier verificación
  automática que se construya en #25.
- **La disponibilidad de la capa gratuita es mala e impredecible.** Dos de
  los cuatro modelos probados no respondieron nunca, y el de respaldo falló
  la mitad de las veces. La demo responderá en modo degradado con
  frecuencia.
- **Google retira modelos.** Los 2.5 ya no existen para cuentas nuevas. Los
  nombres de modelo de este ADR dejarán de funcionar algún día, y el
  síntoma será que todas las consultas pasen a modo degradado.
- **Google usa lo enviado a la capa gratuita para mejorar sus productos**,
  y ahora también las preguntas de quienes usan la demo, no solo los
  documentos. La demo no lo advierte a quien consulta.
- **No se conocen los límites de la cuota de generación.** Google no los
  publica y la API no los informa; las cifras que circulan, de mil a mil
  quinientas respuestas diarias, vienen de terceros.
- **Todo el sistema depende de un solo proveedor**: embeddings, generación
  y respaldo. El respaldo protege de un modelo saturado, no de una caída de
  Gemini ni de un cambio en su capa gratuita.
- **Las mediciones son de un solo día.** La disponibilidad puede ser otra
  mañana, en cualquier dirección.

### Neutras

- Con temperatura cero, las respuestas son casi reproducibles, no
  idénticas.
- Elegir el modelo es una línea de configuración: el contrato de `llmgw` no
  lo nombra.

## Revisión

Reconsiderar si el modo degradado pasa a ser lo habitual, si Google retira
alguno de los dos modelos o cambia las condiciones de su capa gratuita, o si
el conjunto de evaluación (#51) muestra que el modelo agrega contenido que
los pasajes no dicen. La alternativa a evaluar es Claude Haiku 4.5 con saldo
prepagado.
