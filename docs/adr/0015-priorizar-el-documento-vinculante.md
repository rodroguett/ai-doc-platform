# ADR-0015: Priorizar el documento vinculante al ordenar los pasajes

- **Estado:** aceptado
- **Fecha:** 2026-10-18
- **Issue relacionado:** #86

## Contexto

El corpus pasa de un documento a tres del mismo proyecto: la RCA y los dos
ICSARA que la precedieron (#82, #83). Los ICSARA son las preguntas que el
SEA le hizo al titular durante la evaluación; la RCA es la resolución que
la cerró. Tratan los mismos temas, porque la RCA es en buena parte la
respuesta a esas preguntas.

En un sector fiscalizado no valen lo mismo. La RCA es lo vinculante: lo que
el titular debe cumplir y lo que la Superintendencia fiscaliza. Un ICSARA
es historial: una solicitud que pudo acogerse, cambiar o quedar fuera. El
ICSARA pidió avisar a la SMA en menos de 24 horas ante un afloramiento de
aguas, y la RCA no lo recoge.

La evaluación con los tres documentos (#85) midió el efecto de buscarlos
por igual:

- Las preguntas sobre la RCA empeoraron: de 17 de 19 a 15 de 22 con todo
  lo esperado entre los primeros cinco pasajes. Los compromisos
  voluntarios recuperaron uno de cuatro, porque el capítulo VI del ICSARA,
  que pide aclaraciones sobre ellos, ocupó el resto.
- Las tres preguntas que cruzan documentos salieron bien.

Ordenando los mismos resultados con una ventaja para la RCA, sin volver a
generar, se midió:

| Ventaja de la RCA | Preguntas completas | Grupos de secciones |
|---|---|---|
| 0 | 15 de 22 | 30 de 41 |
| 0,01 | 16 de 22 | 33 de 41 |
| 0,02 | 19 de 22 | 37 de 41 |
| 0,03 | 19 de 22 | 38 de 41 |
| 0,04 | 19 de 22 | 38 de 41 |
| 0,05 | 19 de 22 | 38 de 41 |

Con cualquier ventaja, las tres preguntas entre documentos siguen
completas: cuando la pregunta es sobre lo que pidió el SEA, el ICSARA
supera a la RCA por más que eso.

## Decisión

Al ordenar los pasajes, los de un documento vinculante tendrán una ventaja
fija de 0,03 sobre su similitud; el puntaje que se informa y el piso para
citar seguirán siendo la similitud.

**Qué es vinculante.** Un documento cuyo metadato `tipo` es `RCA`. El
metadato lo declara quien sube el documento, igual que su fuente. Un
documento sin `tipo` no tiene ventaja.

**Solo el orden.** La ventaja decide qué pasajes entran entre los primeros;
no cambia su puntaje. La cita muestra la similitud real, y el piso de 0,60
([ADR-0011](0011-recuperacion-y-modo-degradado.md)) se aplica sobre ella:
un pasaje de la RCA que no alcanza el piso no pasa a alcanzarlo.

**0,03 y no más.** Es el menor valor con el mejor resultado medido. Con
más ventaja el resultado no mejora y el riesgo crece: una ventaja grande
haría que la RCA desplazara al ICSARA también en las preguntas sobre lo
que pidió el SEA.

**En la consulta a la base de datos.** El orden se calcula en el mismo
`ORDER BY` de la búsqueda exacta, sin traer pasajes de más para reordenar
después.

## Alternativas consideradas

### Buscar todos los documentos por igual

Es lo que había. Se descartó por lo medido: los ICSARA desplazan a la RCA
justo en las preguntas sobre obligaciones.

### Separar los documentos en colecciones

La API ya permite elegir colección: la RCA en una, los ICSARA en otra. Se
descartó porque deja la decisión a quien consulta, que no sabe dónde está
la respuesta, e impide las preguntas que cruzan documentos.

### Decidir por la pregunta qué documentos buscar

Buscar en los ICSARA solo si la pregunta menciona al SEA, una solicitud o
un ICSARA. Se descartó porque depende de palabras ("pidió", "solicitó")
que una pregunta legítima puede no usar, y falla en silencio.

### Reservar lugares para cada documento

Por ejemplo, tres de los cinco pasajes para la RCA. Se descartó porque
llena esos lugares aunque la RCA no tenga nada pertinente, y le quita
lugar a una pregunta que solo responde un ICSARA.

### Recuperar más pasajes

Subir el `top_k` por omisión de 5 a 8 recupera los pasajes desplazados,
pero cada pasaje son unos 600 tokens más por consulta a todo evento, y el
modelo recibe más material ajeno para mezclar.

### Preferir el documento más reciente

La RCA es posterior a los ICSARA, y un criterio de fecha serviría para
ellos sin declarar tipos. Se descartó porque no generaliza: un informe de
seguimiento es posterior a la RCA y tampoco es vinculante.

## Consecuencias

### Positivas

- Las preguntas sobre obligaciones vuelven a recuperar la RCA, sin perder
  las que cruzan documentos.
- No agrega tokens ni consultas: es un término en el orden de la búsqueda.
- La cita sigue mostrando la similitud real.

### Negativas

- **La ventaja está calibrada con 22 preguntas sobre un proyecto.** Otro
  corpus, u otro modelo de embeddings, puede necesitar otro valor.
- **Depende de un metadato que declara quien sube el documento.** Una RCA
  subida sin `tipo` se busca sin prioridad, y nada lo advierte. La RCA de
  producción se subió sin él y hay que volver a subirla.
- **Una pregunta sobre el historial de la evaluación puede recibir la RCA
  antes que el ICSARA** si los puntajes están a menos de 0,03. Las tres
  preguntas medidas no lo muestran, pero son pocas.
- **No resuelve el estanque de agua tratada,** cuyo pasaje de la RCA
  puntúa bajo aun sin los ICSARA.
- **Un ICSARA puede seguir apareciendo junto a la RCA** en una pregunta
  sobre obligaciones. El modelo lo distingue por su fuente, pero una
  respuesta puede mezclar una solicitud con lo resuelto.

### Neutras

- Un tipo nuevo de documento vinculante, como una resolución que modifique
  la RCA, se agrega a la lista de tipos con prioridad.

## Revisión

Reconsiderar la ventaja si el corpus suma documentos de otro proyecto o de
otro tipo, o si cambia el modelo de embeddings. Reconsiderar el criterio
completo si aparece un documento vinculante que modifique a otro: entonces
importa cuál prevalece, no solo cuál es vinculante.
