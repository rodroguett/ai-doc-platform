# ADR-0011: Recuperar con búsqueda exacta y responder sin generación como modo degradado

- **Estado:** aceptado
- **Fecha:** 2026-10-12
- **Issue relacionado:** #49

## Contexto

Con la ingesta completa, cada fragmento del corpus tiene su vector. Falta
usarlos: dada una pregunta, encontrar los pasajes que la responden y
devolverlos como citas.

El proyecto promete dos cosas que esta etapa tiene que sostener. Que una
respuesta sin fuente verificable no se entrega. Y que el sistema se degrada
de forma controlada cuando el proveedor de generación falla: sigue
respondiendo algo útil en lugar de un error.

Las fuerzas relevantes:

- El corpus es pequeño: una RCA son unos 170 fragmentos.
- Una búsqueda por similitud siempre devuelve "lo más parecido", tenga o no
  relación con la pregunta. No distingue por sí sola una pregunta que el
  corpus responde de una que no.
- Cada consulta nueva gasta una petición del proveedor de embeddings, de
  las mil diarias de su capa gratuita, que comparte con la ingesta
  ([ADR-0008](0008-embeddings-gemini.md)). La API es pública.
- Todavía no hay modelo de generación elegido.

## Decisión

Las consultas se responden recuperando pasajes con una búsqueda exacta por
similitud coseno, se citan solo los que superan un puntaje mínimo, y cuando
no hay texto generado la respuesta lo declara como modo degradado.

**Un módulo `orchestrator` responde las consultas.** Es el que
[ADR-0005](0005-monolito-modular.md) preveía: pide los pasajes a `rag` y,
más adelante, la redacción a `llmgw`. Los tipos de una consulta y de su
respuesta viven en su paquete raíz, y desaparecen los datos de ejemplo.

**`rag` busca; `orchestrator` no ve vectores.** `rag` convierte la pregunta
con el mismo modelo que produjo los vectores guardados, y compara solo
contra los de ese modelo y los de documentos que terminaron su ingesta.

**La búsqueda es exacta, sin índice.** Compara la pregunta contra todos los
vectores. Sobre 5.000 vectores toma 68 milisegundos en la máquina de
desarrollo, y no puede omitir un resultado.

**Un pasaje bajo el puntaje mínimo no se cita.** El piso es 0,60. Si ningún
pasaje lo alcanza, la respuesta dice que no encontró sustento y no trae
citas.

El piso se fijó primero en 0,55, con una medición sobre un extracto de seis
páginas, y se corrigió al medir sobre la RCA completa en el servicio
desplegado:

| Preguntas | Puntajes de los cuatro primeros pasajes |
|---|---|
| Siete que el documento responde | Entre 0,659 y 0,852 |
| Dos ajenas al documento | Entre 0,516 y 0,542 |

En las siete, el primer pasaje fue el que respondía la pregunta. Con el piso
en 0,55, la pregunta ajena mejor puntuada quedó fuera por ocho milésimas;
0,60 queda en medio de la brecha.

**Sin generación, la respuesta entrega los pasajes y lo dice.** La traza
marca `degraded`, con proveedor y modelo `none`, y sin tokens ni costo. No
es un estado transitorio que después se descarta: es el camino que seguirá
toda consulta cuando el proveedor de generación no esté disponible.

**Las preguntas repetidas salen de una caché, y las nuevas tienen un tope
diario.** La caché guarda lo recuperado durante diez minutos; el tope es de
500 consultas nuevas por día. Agotado, las nuevas reciben 429 y las ya
respondidas siguen sirviendo.

## Alternativas consideradas

### Un índice aproximado (HNSW)

Es lo habitual para búsqueda vectorial, y se alcanzó a implementar.

Se quitó porque a esta escala no aporta velocidad perceptible y trae un
costo de corrección: un índice aproximado filtra después de buscar, de modo
que combinado con los filtros por colección, modelo y estado puede devolver
menos pasajes de los pedidos. Es lo primero a agregar si el corpus crece.

### Citar siempre los pasajes más parecidos, sin piso

Es lo más simple y nunca deja una pregunta sin citas.

Se descartó al probarlo: la pregunta "¿cuál es la receta del pisco sour?"
recibió tres citas de la RCA, con puntajes cercanos a 0,5. En un dominio fiscalizado,
una cita que no sustenta nada es peor que ninguna.

### Dejar el piso para cuando exista el conjunto de evaluación

Evitaría fijar un número sin datos suficientes. Se descartó porque el
sistema quedaría mientras tanto citando pasajes sin relación. Se prefiere
un piso provisional y declarado.

### Esperar a tener generación para reemplazar los datos de ejemplo

Se descartó porque los datos de ejemplo eran de un proyecto ficticio, y el
corpus real ya estaba ingerido. Los pasajes reales, aun sin redacción, son
más útiles y más honestos que una respuesta fabricada.

## Consecuencias

### Positivas

- Las consultas devuelven pasajes reales, con su documento, su considerando
  y sus páginas.
- Una pregunta ajena al corpus no recibe citas.
- El modo degradado existe y está probado antes de que exista la
  generación: no será una rama que se escribe al final y nunca se ejercita.
- El repositorio deja de contener datos inventados.

### Negativas

- **El piso de 0,60 sale de nueve preguntas sobre un solo documento.**
  Alcanza para ubicarlo, no para darlo por calibrado. Un piso mal puesto
  falla en silencio hacia ambos lados: descarta pasajes que sí respondían,
  o deja pasar los que no. Las nueve preguntas las escribió quien construyó
  el sistema, conociendo el documento; calibrarlo requiere el conjunto de
  evaluación (#51).
- **Ninguna pregunta medida cae cerca del piso desde arriba.** No se sabe
  cómo puntúa una pregunta pertinente pero mal formulada, o una cuyo
  sustento está repartido en varios pasajes.
- **El piso depende del modelo de embeddings.** Cambiar de modelo o de
  dimensión lo invalida, y nada en el código lo advierte.
- **Sin generación, la respuesta no responde la pregunta**: entrega los
  pasajes para que la persona los lea.
- **La caché y el tope diario viven en memoria.** Se pierden cuando la
  instancia se reinicia o escala a cero, lo que en este servicio ocurre
  tras pocos minutos sin tráfico. El tope real puede ser varias veces 500.
- **La caché puede ocultar por diez minutos un documento recién ingerido**
  a quien repite una pregunta.
- **El tope es global, no por origen.** Un solo cliente puede agotarlo para
  todos, dentro de lo que le permite el límite de peticiones por origen.
- **La búsqueda exacta no escala.** Su costo crece con el corpus; con
  decenas de miles de fragmentos habrá que indexar.
- **Los filtros por metadatos que el contrato declara no se aplican.**
- **Sin base de datos o sin proveedor de embeddings, las consultas
  responden 503.** Antes respondían con datos de ejemplo: `make run` sin
  base de datos ya no muestra nada.

### Neutras

- La traza completa incluye todos los pasajes recuperados, también los que
  no alcanzaron el piso, para poder explicar una respuesta sin citas.
- El extracto de cada cita se acorta a 700 caracteres.

## Revisión

Reconsiderar el piso en cuanto exista el conjunto de evaluación, y cada vez
que cambie el modelo de embeddings. Reconsiderar la búsqueda exacta si la
recuperación supera las décimas de segundo. Reconsiderar la caché y el tope
en memoria si el servicio deja de escalar a cero o pasa a más de una
instancia.
