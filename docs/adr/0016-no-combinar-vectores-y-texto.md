# ADR-0016: No combinar la búsqueda por vectores con la de texto, por ahora

- **Estado:** aceptado
- **Fecha:** 2026-10-18
- **Issue relacionado:** #91

## Contexto

La búsqueda es exacta por similitud coseno, con la prioridad del documento
vinculante de [ADR-0015](0015-priorizar-el-documento-vinculante.md). Sobre
el corpus de tres documentos recupera 19 de 22 preguntas con todo lo
esperado entre los primeros cinco pasajes, y 38 de 41 grupos de secciones.

La falla que más se repite en la evaluación es un pasaje correcto que queda
fuera de los primeros cinco. El caso más claro es `estanque-agua-tratada`:
la 6.2.1 de la RCA queda entre los lugares 6 y 10, y la respuesta habla de
otra cosa. La pregunta repite palabras exactas del pasaje ("volumen útil",
"estanque de acumulación", "4.000 litros/día", "48 horas"), que es el caso
en que una búsqueda por texto suele ganarle a una por vectores.

Postgres trae búsqueda de texto en español (`to_tsvector('spanish', …)`):
combinarla no agrega dependencias ni solicitudes de embeddings.

## Decisión

No combinaremos la búsqueda por vectores con la de texto. La búsqueda sigue
siendo solo por vectores, con la prioridad de ADR-0015.

Se midió antes de construir, sin generar respuestas, sobre los mismos 22
casos respondibles. El texto se buscó con los lexemas de la pregunta unidos
por "o", ordenados por `ts_rank_cd`, y se combinó con el orden por vectores
por fusión de rangos recíprocos (RRF, k = 60), con distintos pesos para el
texto y una ventaja para la RCA aplicada después de fusionar:

| Variante | Completas | Grupos |
|---|---|---|
| Solo vectores (vigente) | **19** | **38 de 41** |
| RRF, texto con peso 1 | 15 | 30 |
| RRF, texto con peso 0,5 | 17 | 32 |
| RRF, texto 0,5 y ventaja RCA 0,004 | 19 | 36 |
| RRF, texto 0,25 y ventaja RCA 0,002 | 19 | 37 |
| RRF, texto 0,25 y ventaja RCA 0,004 | 17 | 34 |
| RRF, texto 0,1 | 19 | 37 |

**El texto sí resuelve el estanque**: pone la 6.2.1 primera, y todas las
variantes con fusión la recuperan. Pero las enumeraciones empeoran: las
solicitudes del ICSARA usan las mismas palabras que la RCA ("compromiso",
"plan de monitoreo", "permiso"), y el texto las sube. La mejor combinación
iguala a los vectores en preguntas completas y queda por debajo en grupos:
cambia el estanque por los permisos con la sigla o por las fechas.

Ninguna variante mejora, y las que empatan agregan dos parámetros
calibrados con 22 preguntas, que es casi un parámetro cada diez. Además
son frágiles: con texto 0,25, subir la ventaja de la RCA de 0,002 a 0,004
rompe dos de las tres preguntas que cruzan documentos.

## Alternativas consideradas

Las de la tabla. No se probaron, y quedan como lo siguiente a medir si el
problema vuelve:

- **Usar el texto solo para desempatar** entre pasajes de similitud
  parecida, en vez de fusionar órdenes.
- **Reordenar los primeros veinte con un modelo de reordenamiento.** Es lo
  que más probablemente resuelve el caso, pero cuesta una llamada más por
  consulta y no hay uno en la capa gratuita del proveedor actual.
- **Buscar el texto con frases** en vez de lexemas sueltos, para que
  "estanque de acumulación" pese como frase.

## Consecuencias

### Positivas

- La búsqueda sigue con un solo mecanismo y un solo parámetro (la ventaja
  de la RCA), calibrado con el mismo conjunto.
- Queda medido que el texto sí encuentra el estanque: si se retoma, se
  parte de este resultado y no de cero.

### Negativas

- **`estanque-agua-tratada` sigue sin respuesta útil.** Es una pregunta
  real de revisión crítica, y el pasaje que la responde existe.
- **La conclusión depende de 22 preguntas de un proyecto.** Con un corpus
  donde las preguntas usen más términos exactos, o menos documentos que se
  repiten entre sí, la combinación podría ganar.
- **La medición fusionó solo los primeros 50 pasajes por vectores.** Un
  pasaje que el texto encuentra y los vectores no ponen entre los 50 no se
  consideró. Con 446 fragmentos en el corpus, no cambia la conclusión, pero
  no está medido.

## Revisión

Reconsiderar si el corpus crece con documentos de otro tipo, si la
evaluación suma preguntas con términos exactos que hoy fallan, o si aparece
un modelo de reordenamiento sin costo.
