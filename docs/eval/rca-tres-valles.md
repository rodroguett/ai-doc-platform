# Evaluación de la búsqueda: internal/rag/testdata/rca-tres-valles.pdf

- **Fecha:** 2026-10-14
- **Modelo de embeddings:** `gemini-embedding-001`
- **Puntaje mínimo para citar:** 0.60
- **Preguntas:** 17 (13 respondibles, 4 que el documento no responde)

Cada pregunta mide un comportamiento distinto. Las de origen 'autor' las escribió quien construyó el sistema, conociendo el documento: están sesgadas hacia lo que el sistema hace bien. Las de origen 'asistente' las generó un asistente de IA a pedido del dueño del proyecto, y él las seleccionó; algunas traen premisas falsas, a propósito o no.

## Resultado

| Medida | Valor |
|---|---|
| Preguntas con todo lo esperado entre los primeros 5 pasajes | 10 de 13 |
| Ídem, y además con puntaje suficiente para citarse | 10 de 13 |
| Grupos de secciones esperadas recuperados entre los primeros 5 | 20 de 24 |
| Ídem entre los primeros 10 | 23 de 24 |
| Preguntas sin respuesta en el documento que recibirían citas | 3 de 4 |

## Por pregunta

| Pregunta | Origen | Esperado | Recuperado (primeros 5) | Resultado |
|---|---|---|---|---|
| `aire-parametros` | autor | Considerando 10.1 | Considerando 10.1 (0.735)<br>Considerando 8.1 (0.696)<br>Considerando 10.2 (0.663)<br>Considerando 10.3 (0.659)<br>Considerando 7.1 (0.655) | 1/1 grupos en los primeros 5 (1/1 en 10), 1 citables |
| `informes-frecuencia` | autor | Considerando 10.1 o Considerando 8.1 | Considerando 10.1 (0.719)<br>Considerando 8.1 (0.696)<br>Considerando 15 (0.672)<br>Considerando 10.2 (0.662)<br>Considerando 7.1 (0.660) | 1/1 grupos en los primeros 5 (1/1 en 10), 1 citables |
| `vida-util` | autor | Considerando 4.3.3 | Considerando 4.3.3 (0.805)<br>Considerando 4.4.4 (0.741)<br>Considerando 4.3 (0.712)<br>Considerando 4.4.2 (0.711)<br>Considerando 4.3 (0.705) | 1/1 grupos en los primeros 5 (1/1 en 10), 1 citables |
| `permisos-sectoriales` | autor | Considerando 6.1; Considerando 6.2 o Considerando 6.2.1 o Considerando 6.2.2 | Considerando 6 (0.823)<br>Considerando 6.2 (0.796)<br>Considerando 6.1 (0.793)<br>Considerando 6.2.2 (0.773)<br>Resuelvo 3 (0.750) | 2/2 grupos en los primeros 5 (2/2 en 10), 2 citables |
| `sismo` | autor | Considerando 12.1 | Considerando 12.1 (0.708)<br>Considerando 12.1 (0.679)<br>Considerando 12.3 (0.651)<br>Considerando 12.6 (0.618)<br>Considerando 20 (0.611) | 1/1 grupos en los primeros 5 (1/1 en 10), 1 citables |
| `inversion` | autor | Considerando 4.1 | Considerando 4.1 (0.730)<br>Considerando 9 (0.664)<br>Considerando 8 (0.660)<br>Considerando 4.3 (0.660)<br>Considerando 4.4.4 (0.655) | 1/1 grupos en los primeros 5 (1/1 en 10), 1 citables |
| `compromisos-voluntarios` | autor | Considerando 8.1; Considerando 8.2; Considerando 8.3; Considerando 8.4 | Considerando 8 (0.852)<br>Considerando 8.1 (0.739)<br>Considerando 11.1 (0.738)<br>Considerando 10 (0.728)<br>Considerando 8.2 (0.724) | 2/4 grupos en los primeros 5 (4/4 en 10), 2 citables |
| `estanque-agua-tratada` | asistente | Considerando 6.2.1 | Considerando 6.2.1 (0.641)<br>Considerando 4.3 (0.618)<br>Considerando 4.3 (0.617)<br>Considerando 4.3 (0.612)<br>Considerando 5.2 (0.611) | 1/1 grupos en los primeros 5 (1/1 en 10), 1 citables |
| `estanque-tk65` | asistente | Considerando 4.3 o Considerando 7.5 | Considerando 7.5 (0.703)<br>Considerando 4.3 (0.701)<br>Considerando 7.5 (0.701)<br>Considerando 7.4 (0.646)<br>Considerando 7.4 (0.639) | 1/1 grupos en los primeros 5 (1/1 en 10), 1 citables |
| `fechas-expediente` | asistente | Vistos 1; Considerando 2 o Considerando 3; Considerando 13 o Considerando 14 | Considerando 14 (0.697)<br>Vistos 4 (0.696)<br>Considerando 13 (0.688)<br>Vistos 2 (0.674)<br>Considerando 2 (0.665) | 2/3 grupos en los primeros 5 (3/3 en 10), 2 citables |
| `limites-genericos` | asistente | Considerando 8.2; Considerando 8.3; Considerando 10.2; Considerando 10.3 | Considerando 10.2 (0.704)<br>Considerando 10.3 (0.700)<br>Considerando 8.3 (0.695)<br>Considerando 8.2 (0.693)<br>Considerando 10.1 (0.663) | 4/4 grupos en los primeros 5 (4/4 en 10), 4 citables |
| `norma-aplicable` | asistente | Considerando 8.2 o Considerando 8.3 o Considerando 10.2 o Considerando 10.3 | Considerando 15 (0.696)<br>Considerando 7.1 (0.695)<br>Considerando 7.1 (0.686)<br>Considerando 7.4 (0.682)<br>Considerando 18 (0.681) | 0/1 grupos en los primeros 5 (0/1 en 10), 0 citables |
| `plazos-contingencias` | asistente | Considerando 12.1; Considerando 12.2; Considerando 12.3 o Considerando 12.4 o Considerando 12.5 o Considerando 12.6 o Considerando 12.7 o Considerando 12.8 o Considerando 12.9 | Considerando 12.1 (0.743)<br>Considerando 12.1 (0.728)<br>Considerando 12.6 (0.697)<br>Considerando 12.3 (0.695)<br>Considerando 12.2 (0.684) | 3/3 grupos en los primeros 5 (3/3 en 10), 3 citables |
| `sin-respuesta-precio-cobre` | autor | — | Considerando 4.3 (0.665)<br>Considerando 4.1 (0.661)<br>Considerando 4.3 (0.649)<br>Considerando 4.3 (0.647)<br>Considerando 4.3 (0.641) | no respondible: primer puntaje 0.665, CITARÍA |
| `sin-respuesta-sueldos` | autor | — | Considerando 4.4.4 (0.688)<br>Considerando 4.4.2 (0.647)<br>Considerando 4.3 (0.627)<br>Considerando 4.3 (0.626)<br>Considerando 5.3 (0.625) | no respondible: primer puntaje 0.688, CITARÍA |
| `sin-respuesta-energia-solar` | autor | — | Considerando 8 (0.633)<br>Considerando 4.3 (0.620)<br>Considerando 4.3 (0.618)<br>Considerando 9 (0.617)<br>Considerando 4.3 (0.617) | no respondible: primer puntaje 0.633, CITARÍA |
| `sin-respuesta-ajena` | autor | — | Considerando 4.3 (0.542)<br>Resuelvo 6 (0.542)<br>Considerando 4.3 (0.541)<br>Considerando 4.3 (0.534)<br>Considerando 23 (0.533) | no respondible: primer puntaje 0.542, sin citas |

## Puntaje mínimo

Para cada umbral: cuántas preguntas respondibles tendrían su mejor pasaje esperado en condiciones de citarse, y cuántas sin respuesta recibirían citas.

| Umbral | Respondibles con sustento citable | Sin respuesta que recibirían citas |
|---|---|---|
| 0.500 | 12 de 13 | 4 de 4 |
| 0.525 | 12 de 13 | 4 de 4 |
| 0.550 | 12 de 13 | 3 de 4 |
| 0.575 | 12 de 13 | 3 de 4 |
| 0.600 **(actual)** | 12 de 13 | 3 de 4 |
| 0.625 | 12 de 13 | 3 de 4 |
| 0.650 | 11 de 13 | 2 de 4 |
| 0.675 | 11 de 13 | 1 de 4 |
| 0.700 | 10 de 13 | 0 de 4 |
| 0.725 | 6 de 13 | 0 de 4 |
| 0.750 | 2 de 13 | 0 de 4 |

## Respuestas redactadas, para revisión manual

La verificación automática comprueba que las citas existan, no que la respuesta sea correcta. Estas respuestas se revisan a mano contra lo que cada pregunta mide.

### `aire-parametros`

**Mide:** Un dato con varias partes en una sola sección

**Pregunta:** ¿Qué parámetros de calidad del aire se monitorean y con qué límites?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> Los parámetros de calidad del aire que se monitorean son material particulado respirable (MP-10), material particulado sedimentable (MPS) y parámetros meteorológicos (velocidad y dirección del viento, temperatura, humedad relativa y pluviometría) [1][2]. Los límites máximos comprometidos para el MP-10 son un valor promedio diario de 150 ug/m3N y un valor promedio trianual de 50 ug/m3N, mientras que para el MPS se utiliza como referencia la norma establecida en el Decreto Supremo N°4/1992 del Ministerio de Salud [1].

1. Considerando 10.1, págs. 89-90 (0.735)
2. Considerando 8.1, pág. 86 (0.696)

### `informes-frecuencia`

**Mide:** El mismo dato dicho de dos formas en dos secciones ('cada 6 meses', 'semestral')

**Pregunta:** ¿Cada cuánto se entregan los informes de seguimiento de calidad del aire a la autoridad?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> Los informes de seguimiento del plan de estaciones de monitoreo de calidad del aire deben entregarse a la autoridad competente cada 6 meses [1][2].

1. Considerando 10.1, págs. 89-90 (0.719)
2. Considerando 8.1, pág. 86 (0.696)

### `vida-util`

**Mide:** Un dato puntual en una sección larga

**Pregunta:** ¿Cuál es la vida útil del proyecto y cuánto dura la fase de cierre?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> El proyecto contempla una vida útil de 11 años de operación, mientras que la fase de cierre tiene una duración estimada de tres años [1].

1. Considerando 4.3.3, pág. 35 (0.805)

### `permisos-sectoriales`

**Mide:** Una respuesta que combina subsecciones hermanas

**Pregunta:** ¿Qué permisos ambientales sectoriales aplican al proyecto?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> Al proyecto le son aplicables permisos ambientales sectoriales mixtos [1][2], específicamente aquellos contenidos en los artículos 138 y 160 del Reglamento del SEIA [5], destacando el permiso del artículo 160 para construcciones fuera de los límites urbanos [4], mientras que el proyecto no contempla permisos de contenido únicamente ambiental [3].

1. Considerando 6, pág. 61 (0.823)
2. Considerando 6.2, pág. 61 (0.796)
3. Considerando 6.1, pág. 61 (0.793)
4. Considerando 6.2.2, págs. 63-64 (0.773)
5. Resuelvo 3, pág. 107 (0.750)

### `sismo`

**Mide:** Una sección dividida en varios fragmentos

**Pregunta:** ¿Qué debe hacer el titular ante un sismo?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> Ante un sismo, el titular debe implementar medidas de prevención como diseñar instalaciones bajo criterios sismo-resistentes, capacitar al personal y difundir las áreas de seguridad [1]. Durante el evento, los trabajadores deben mantener la calma, dirigirse a la zona de seguridad más cercana, realizar un recuento del personal y verificar si existen lesionados o daños materiales que puedan provocar emergencias ambientales [1].

1. Considerando 12.1, págs. 93-94 (0.708)

### `inversion`

**Mide:** Una cifra dentro de una sección de antecedentes generales

**Pregunta:** ¿Cuál es el monto de inversión del proyecto?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> La inversión estimada para la ejecución del Proyecto será de USD $ 30 millones de dólares americanos [1].

1. Considerando 4.1, págs. 3-4 (0.730)

### `compromisos-voluntarios`

**Mide:** Una enumeración: hay que recuperar las cuatro subsecciones. Falló en producción al verificar #25

**Pregunta:** ¿Qué compromisos ambientales voluntarios contempla el proyecto?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> El proyecto contempla realizar el monitoreo de la calidad del aire [1][2], el plan de monitoreo de la calidad de aguas subterráneas [1][4] y la charla de difusión del proyecto [1][3].

1. Considerando 8, pág. 86 (0.852)
2. Considerando 8.1, pág. 86 (0.739)
3. Considerando 11.1, págs. 92-93 (0.738)
4. Considerando 8.2, págs. 86-87 (0.724)

### `estanque-agua-tratada`

**Mide:** El documento reconoce su propia incongruencia y fija la exigencia, pero no explica la causa: la respuesta no debe inventarla

**Pregunta:** ¿A qué se deben las incongruencias en el volumen útil del estanque de acumulación de agua tratada mencionadas en el texto, y se encuentra debidamente garantizado que contará con una capacidad mínima de 4.000 litros/día y un tiempo de retención no superior a 48 horas?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> Los pasajes entregados no explican a qué se deben las incongruencias en el volumen del estanque, pero sí indican que el Titular deberá garantizar que este cuente con un volumen útil mínimo equivalente al caudal de diseño de aguas servidas (4.000 litros/día) y que su tiempo de retención no exceda las 48 horas [1].

1. Considerando 6.2.1, págs. 62-63 (0.641)

### `estanque-tk65`

**Mide:** Premisa falsa: el documento dice 34 m3 en ambas secciones y '4 m3' no aparece. La respuesta no debe confirmar la contradicción

**Pregunta:** ¿Cuál es la capacidad real y definitiva del estanque TK-65 (destinado a xileno/diluyente), dado que en la descripción del numeral 4.3.2 se señalan 34 m3, mientras que en la Tabla 7.5.1 sobre cumplimiento del DS 594 se indica 4 m3?

**Respuesta** (modo degradado):

> No hay una respuesta redactada para esta consulta. Estos son los 5 pasajes del corpus más relacionados con la pregunta, cada uno con su documento y su ubicación.

1. Considerando 7.5, págs. 76-77 (0.703)
2. Considerando 4.3, págs. 34-35 (0.701)
3. Considerando 7.5, págs. 77-78 (0.701)
4. Considerando 7.4, págs. 73-74 (0.646)
5. Considerando 7.4, págs. 74-75 (0.639)

### `fechas-expediente`

**Mide:** Fechas repartidas en tres secciones, con una premisa sesgada: la secuencia es la normal de una evaluación

**Pregunta:** ¿Por qué existe un desfase temporal en la secuencia del expediente de evaluación, figurando un proceso de Participación Ciudadana (PAC) desarrollado entre octubre y noviembre de 2024, pero citando Adendas presentadas en abril y noviembre de 2025 e Informes Consolidados de Evaluación (ICE) de diciembre de 2025?

**Respuesta** (modo degradado):

> No hay una respuesta redactada para esta consulta. Estos son los 5 pasajes del corpus más relacionados con la pregunta, cada uno con su documento y su ubicación.

1. Considerando 14, pág. 106 (0.697)
2. Vistos 4, pág. 1 (0.696)
3. Considerando 13, pág. 106 (0.688)
4. Vistos 2, pág. 1 (0.674)
5. Considerando 2, pág. 2 (0.665)

### `limites-genericos`

**Mide:** Un hecho cierto repartido en cuatro secciones; el motivo no está en el documento

**Pregunta:** ¿Por qué en los Planes de Monitoreo de Calidad de Aguas Subterráneas y Superficiales (numerales 8.2, 8.3, 10.2 y 10.3) se omite el valor normativo numérico o la norma específica de referencia, utilizando la fórmula genérica "límites máximos permitidos por la autoridad correspondiente"?

**Respuesta** (modo degradado):

> No hay una respuesta redactada para esta consulta. Estos son los 5 pasajes del corpus más relacionados con la pregunta, cada uno con su documento y su ubicación.

1. Considerando 10.2, pág. 90 (0.704)
2. Considerando 10.3, págs. 90-91 (0.700)
3. Considerando 8.3, págs. 87-88 (0.695)
4. Considerando 8.2, págs. 86-87 (0.693)
5. Considerando 10.1, págs. 89-90 (0.663)

### `norma-aplicable`

**Mide:** Juicio jurídico que el documento no hace. Trampa: el D.S. 90/2000 aparece, pero para la descarga de riles; la respuesta no debe atribuirlo a los planes de monitoreo

**Pregunta:** ¿Cómo se garantiza la certeza jurídica y la fiscalizabilidad de estos compromisos ante la Superintendencia del Medio Ambiente (SMA) si no se estipula de forma explícita la norma aplicable (ej. NCh 1333 o D.S. N° 90/2000)?

**Respuesta** (modo degradado):

> No hay una respuesta redactada para esta consulta. Estos son los 5 pasajes del corpus más relacionados con la pregunta, cada uno con su documento y su ubicación.

1. Considerando 15, pág. 107 (0.696)
2. Considerando 7.1, págs. 67-68 (0.695)
3. Considerando 7.1, págs. 66-67 (0.686)
4. Considerando 7.4, págs. 75-76 (0.682)
5. Considerando 18, pág. 107 (0.681)

### `plazos-contingencias`

**Mide:** Premisa en parte falsa: las 24 horas aplican también a condiciones climáticas (12.2). Hay que juntar pasajes de varias contingencias para corregirla

**Pregunta:** ¿Por qué el Plan de Prevención de Contingencias y Emergencias fija un plazo de notificación a la SMA de 24 horas en caso de evento sísmico (numeral 12.1), mientras que para el resto de las contingencias (climáticas, remoción en masa, inundación, incendios, filtraciones, etc.) se establece un plazo de 48 horas?

**Respuesta** (modo degradado):

> No hay una respuesta redactada para esta consulta. Estos son los 5 pasajes del corpus más relacionados con la pregunta, cada uno con su documento y su ubicación.

1. Considerando 12.1, pág. 94 (0.743)
2. Considerando 12.1, págs. 93-94 (0.728)
3. Considerando 12.6, pág. 99 (0.697)
4. Considerando 12.3, págs. 95-96 (0.695)
5. Considerando 12.2, pág. 95 (0.684)

### `sin-respuesta-precio-cobre`

**Mide:** Del dominio, pero ausente del documento: no debe recibir citas

**Pregunta:** ¿Qué precio del cobre se consideró en la evaluación económica del proyecto?

**Respuesta** (modo degradado):

> No hay una respuesta redactada para esta consulta. Estos son los 5 pasajes del corpus más relacionados con la pregunta, cada uno con su documento y su ubicación.

1. Considerando 4.3, págs. 27-28 (0.665)
2. Considerando 4.1, págs. 2-3 (0.661)
3. Considerando 4.3, págs. 25-26 (0.649)
4. Considerando 4.3, págs. 24-25 (0.647)
5. Considerando 4.3, págs. 14-15 (0.641)

### `sin-respuesta-sueldos`

**Mide:** Del dominio, pero ausente del documento: no debe recibir citas

**Pregunta:** ¿Cuál será el sueldo promedio de los trabajadores durante la operación?

**Respuesta** (modo degradado):

> No hay una respuesta redactada para esta consulta. Estos son los 5 pasajes del corpus más relacionados con la pregunta, cada uno con su documento y su ubicación.

1. Considerando 4.4.4, págs. 40-41 (0.688)
2. Considerando 4.4.2, pág. 40 (0.647)
3. Considerando 4.3, pág. 7 (0.627)
4. Considerando 4.3, págs. 10-12 (0.626)
5. Considerando 5.3, págs. 56-57 (0.625)

### `sin-respuesta-energia-solar`

**Mide:** Del dominio, pero ausente del documento: no debe recibir citas

**Pregunta:** ¿Cuántos paneles solares instalará el proyecto para abastecer la planta?

**Respuesta** (modo degradado):

> No hay una respuesta redactada para esta consulta. Estos son los 5 pasajes del corpus más relacionados con la pregunta, cada uno con su documento y su ubicación.

1. Considerando 8, pág. 86 (0.633)
2. Considerando 4.3, págs. 14-15 (0.620)
3. Considerando 4.3, págs. 17-19 (0.618)
4. Considerando 9, pág. 88 (0.617)
5. Considerando 4.3, pág. 17 (0.617)

### `sin-respuesta-ajena`

**Mide:** Ajena al dominio: no debe recibir citas

**Pregunta:** ¿Cuál es la receta del pisco sour?

**Respuesta** (sin sustento):

> No se encontraron pasajes en el corpus que sustenten una respuesta a esta pregunta.


