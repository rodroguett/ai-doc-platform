# Evaluación de la búsqueda: internal/rag/testdata/rca-tres-valles.pdf

- **Fecha:** 2026-10-15
- **Modelo de embeddings:** `gemini-embedding-001`
- **Puntaje mínimo para citar:** 0.60
- **Preguntas:** 19 (15 respondibles, 4 que el documento no responde)

Cada pregunta mide un comportamiento distinto. Las de origen 'autor' las escribió quien construyó el sistema, conociendo el documento: están sesgadas hacia lo que el sistema hace bien. Las de origen 'asistente' las generó un asistente de IA a pedido del dueño del proyecto, y quedaron las que aportaban; algunas traen premisas falsas, a propósito o no.

## Resultado

| Medida | Valor |
|---|---|
| Preguntas con todo lo esperado entre los primeros 5 pasajes | 13 de 15 |
| Ídem, y además con puntaje suficiente para citarse | 13 de 15 |
| Grupos de secciones esperadas recuperados entre los primeros 5 | 30 de 32 |
| Ídem entre los primeros 10 | 31 de 32 |
| Preguntas sin respuesta en el documento que recibirían citas | 3 de 4 |
| Respuestas que citan una sección que no corresponde | 0 de 2 |

## Por pregunta

| Pregunta | Origen | Esperado | Recuperado (primeros 5) | Resultado |
|---|---|---|---|---|
| `aire-parametros` | autor | Considerando 10.1 | Considerando 10.1 (0.725)<br>Considerando 8.1 (0.680)<br>Considerando 10.2 (0.665)<br>Considerando 10.3 (0.664)<br>Considerando 7.1 (0.654) | 1/1 grupos en los primeros 5 (1/1 en 10), 1 citables |
| `informes-frecuencia` | autor | Considerando 10.1 o Considerando 8.1 | Considerando 10.1 (0.710)<br>Considerando 10.2 (0.672)<br>Considerando 15 (0.672)<br>Considerando 8.1 (0.667)<br>Considerando 7.1 (0.666) | 1/1 grupos en los primeros 5 (1/1 en 10), 1 citables |
| `vida-util` | autor | Considerando 4.3.3 | Considerando 4.3.3 (0.794)<br>Considerando 4.4.4 (0.732)<br>Considerando 4.3 (0.724)<br>Considerando 4.4.2 (0.717)<br>Considerando 4.3 (0.708) | 1/1 grupos en los primeros 5 (1/1 en 10), 1 citables |
| `permisos-sectoriales` | autor | Considerando 6.1; Considerando 6.2.1; Considerando 6.2.2 | Considerando 6.2 (0.815)<br>Considerando 6.2.1 (0.813)<br>Considerando 6.1 (0.809)<br>Considerando 6.2.1 (0.789)<br>Considerando 6.2.2 (0.788) | 3/3 grupos en los primeros 5 (3/3 en 10), 3 citables |
| `permisos-sectoriales-cuales` | autor | Considerando 6.1; Considerando 6.2.1; Considerando 6.2.2 | Considerando 6.2 (0.790)<br>Considerando 6.1 (0.780)<br>Considerando 6.2.1 (0.775)<br>Considerando 6.2.1 (0.771)<br>Considerando 6.2.2 (0.764) | 3/3 grupos en los primeros 5 (3/3 en 10), 3 citables |
| `sismo` | autor | Considerando 12.1 | Considerando 12.1 (0.697)<br>Considerando 12.1 (0.657)<br>Considerando 12.3 (0.649)<br>Considerando 20 (0.611)<br>Considerando 12.6 (0.610) | 1/1 grupos en los primeros 5 (1/1 en 10), 1 citables |
| `inversion` | autor | Considerando 4.1 | Considerando 4.1 (0.720)<br>Considerando 9 (0.664)<br>Considerando 4.4.1 (0.663)<br>Considerando 4.4.4 (0.661)<br>Considerando 4.4.2 (0.656) | 1/1 grupos en los primeros 5 (1/1 en 10), 1 citables |
| `compromisos-voluntarios` | autor | Considerando 8.1; Considerando 8.2; Considerando 8.3; Considerando 8.4 | Considerando 8.1 (0.783)<br>Considerando 8.2 (0.770)<br>Considerando 8.4 (0.762)<br>Considerando 8.3 (0.760)<br>Considerando 11.1 (0.745) | 4/4 grupos en los primeros 5 (4/4 en 10), 4 citables |
| `compromisos-voluntarios-cuales` | autor | Considerando 8.1; Considerando 8.2; Considerando 8.3; Considerando 8.4 | Considerando 8.1 (0.783)<br>Considerando 8.2 (0.767)<br>Considerando 8.4 (0.762)<br>Considerando 8.3 (0.758)<br>Considerando 11.1 (0.742) | 4/4 grupos en los primeros 5 (4/4 en 10), 4 citables |
| `estanque-agua-tratada` | asistente | Considerando 6.2.1 | Considerando 4.3 (0.625)<br>Considerando 4.3 (0.621)<br>Considerando 6.2.1 (0.617)<br>Considerando 4.3 (0.613)<br>Considerando 4.3 (0.611) | 1/1 grupos en los primeros 5 (1/1 en 10), 1 citables |
| `estanque-tk65` | asistente | Considerando 4.3 o Considerando 7.5 | Considerando 4.3 (0.716)<br>Considerando 7.5 (0.703)<br>Considerando 7.5 (0.703)<br>Considerando 7.1 (0.645)<br>Considerando 7.4 (0.643) | 1/1 grupos en los primeros 5 (1/1 en 10), 1 citables |
| `fechas-expediente` | asistente | Vistos 1; Considerando 2 o Considerando 3; Considerando 13 o Considerando 14 | Vistos 4 (0.691)<br>Considerando 14 (0.689)<br>Considerando 13 (0.677)<br>Vistos 2 (0.673)<br>Considerando 2 (0.657) | 2/3 grupos en los primeros 5 (3/3 en 10), 2 citables |
| `limites-genericos` | asistente | Considerando 8.2; Considerando 8.3; Considerando 10.2; Considerando 10.3 | Considerando 10.2 (0.706)<br>Considerando 10.3 (0.705)<br>Considerando 8.3 (0.695)<br>Considerando 8.2 (0.691)<br>Considerando 10.1 (0.662) | 4/4 grupos en los primeros 5 (4/4 en 10), 4 citables |
| `norma-aplicable` | asistente | Considerando 8.2 o Considerando 8.3 o Considerando 10.2 o Considerando 10.3 | Considerando 7.1 (0.709)<br>Considerando 7.2 (0.705)<br>Considerando 7.1 (0.703)<br>Considerando 7.1 (0.703)<br>Considerando 7.1 (0.698) | 0/1 grupos en los primeros 5 (0/1 en 10), 0 citables |
| `plazos-contingencias` | asistente | Considerando 12.1; Considerando 12.2; Considerando 12.3 o Considerando 12.4 o Considerando 12.5 o Considerando 12.6 o Considerando 12.7 o Considerando 12.8 o Considerando 12.9 | Considerando 12.1 (0.792)<br>Considerando 12.1 (0.734)<br>Considerando 12.6 (0.714)<br>Considerando 12.2 (0.706)<br>Considerando 12.3 (0.702) | 3/3 grupos en los primeros 5 (3/3 en 10), 3 citables |
| `sin-respuesta-precio-cobre` | autor | — | Considerando 4.3 (0.666)<br>Considerando 4.1 (0.659)<br>Considerando 4.3 (0.656)<br>Considerando 4.4.2 (0.649)<br>Considerando 4.3 (0.649) | no respondible: primer puntaje 0.666, CITARÍA |
| `sin-respuesta-sueldos` | autor | — | Considerando 4.4.4 (0.680)<br>Considerando 4.4.2 (0.641)<br>Considerando 4.3.2 (0.623)<br>Considerando 5.3 (0.617)<br>Considerando 4.3 (0.614) | no respondible: primer puntaje 0.680, CITARÍA |
| `sin-respuesta-energia-solar` | autor | — | Considerando 4.3 (0.623)<br>Considerando 4.3 (0.619)<br>Considerando 4.3 (0.619)<br>Considerando 9 (0.617)<br>Considerando 4.3 (0.617) | no respondible: primer puntaje 0.623, CITARÍA |
| `sin-respuesta-ajena` | autor | — | Resuelvo 6 (0.542)<br>Considerando 4.3 (0.540)<br>Considerando 4.3 (0.539)<br>Considerando 4.3 (0.538)<br>Considerando 23 (0.533) | no respondible: primer puntaje 0.542, sin citas |

## Puntaje mínimo

Para cada umbral: cuántas preguntas respondibles tendrían su mejor pasaje esperado en condiciones de citarse, y cuántas sin respuesta recibirían citas.

| Umbral | Respondibles con sustento citable | Sin respuesta que recibirían citas |
|---|---|---|
| 0.500 | 14 de 15 | 4 de 4 |
| 0.525 | 14 de 15 | 4 de 4 |
| 0.550 | 14 de 15 | 3 de 4 |
| 0.575 | 14 de 15 | 3 de 4 |
| 0.600 **(actual)** | 14 de 15 | 3 de 4 |
| 0.625 | 13 de 15 | 2 de 4 |
| 0.650 | 13 de 15 | 2 de 4 |
| 0.675 | 13 de 15 | 1 de 4 |
| 0.700 | 11 de 15 | 0 de 4 |
| 0.725 | 6 de 15 | 0 de 4 |
| 0.750 | 6 de 15 | 0 de 4 |

## Respuestas redactadas, para revisión manual

La verificación automática comprueba que las citas existan, no que la respuesta sea correcta. Estas respuestas se revisan a mano contra lo que cada pregunta mide.

### `aire-parametros`

**Mide:** Un dato con varias partes en una sola sección

**Pregunta:** ¿Qué parámetros de calidad del aire se monitorean y con qué límites?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> Para caracterizar el estado y evolución de la calidad del aire se monitorean el material particulado respirable (MP-10), el material particulado sedimentable (MPS) y los parámetros meteorológicos, los cuales incluyen velocidad y dirección del viento, temperatura, humedad relativa y pluviometría [1][2]. Los límites comprometidos para el material particulado respirable MP-10 son un valor máximo promedio diario de 150 ug/m3N y un valor promedio trianual de 50 ug/m3N, mientras que para el material particulado sedimentable (MPS) el límite corresponde a la norma de referencia establecida en el Decreto Supremo N°4/1992 del Ministerio de Salud [1].

1. Considerando 10.1, págs. 89-90 (0.725)
2. Considerando 8.1, pág. 86 (0.680)

### `informes-frecuencia`

**Mide:** El mismo dato dicho de dos formas en dos secciones ('cada 6 meses', 'semestral')

**Pregunta:** ¿Cada cuánto se entregan los informes de seguimiento de calidad del aire a la autoridad?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> Los informes de seguimiento de calidad del aire deben entregarse a la autoridad competente cada 6 meses [1][2].

1. Considerando 10.1, págs. 89-90 (0.710)
2. Considerando 8.1, pág. 86 (0.667)

### `vida-util`

**Mide:** Un dato puntual en una sección larga

**Pregunta:** ¿Cuál es la vida útil del proyecto y cuánto dura la fase de cierre?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> El proyecto contempla una vida útil de 11 años de operación [1]. Por su parte, la fase de cierre tiene una duración estimada de tres años [1][2].

1. Considerando 4.3.3, pág. 35 (0.794)
2. Considerando 4.3, pág. 36 (0.724)

### `permisos-sectoriales`

**Mide:** Una respuesta que combina subsecciones hermanas: cada permiso está en la suya

**Pregunta:** ¿Qué permisos ambientales sectoriales aplican al proyecto?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> Los permisos ambientales sectoriales aplicables al proyecto son los permisos ambientales sectoriales mixtos [1]. Dentro de estos se encuentra el permiso para la construcción, reparación, modificación y ampliación de cualquier obra pública o particular destinada a la evacuación, tratamiento o disposición final de desagües y aguas servidas (establecido en el artículo 138 del Reglamento del SEIA) [3], y el permiso para subdividir y urbanizar terrenos rurales o para construcciones fuera de los límites urbanos (establecido en el artículo 160 del Reglamento del SEIA) [4]. El proyecto no contempla permisos ambientales sectoriales de contenido únicamente ambiental [2].

1. Considerando 6.2, pág. 61 (0.815)
2. Considerando 6.1, pág. 61 (0.809)
3. Considerando 6.2.1, págs. 62-63 (0.789)
4. Considerando 6.2.2, págs. 63-64 (0.788)

### `permisos-sectoriales-cuales`

**Mide:** La misma enumeración, con la sigla con que la escribiría alguien del sector. Sin expandir la sigla, la búsqueda dejaba fuera la 6.2.2 y la respuesta omitía el permiso del artículo 160 (#62)

**Pregunta:** ¿Cuáles son los PAS del proyecto?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> El proyecto no contempla permisos ambientales sectoriales de contenido únicamente ambiental [2]. Respecto a los permisos ambientales sectoriales mixtos aplicables al proyecto, estos corresponden al permiso para la construcción, reparación, modificación y ampliación de cualquier obra pública o particular destinada a la evacuación, tratamiento o disposición final de desagües, aguas servidas de cualquier naturaleza según el artículo 138 del Reglamento del SEIA, y al permiso para subdividir y urbanizar terrenos rurales o para construcciones fuera de los límites urbanos según el artículo 160 del Reglamento del SEIA [1][3][4].

1. Considerando 6.2, pág. 61 (0.790)
2. Considerando 6.1, pág. 61 (0.780)
3. Considerando 6.2.1, págs. 62-63 (0.771)
4. Considerando 6.2.2, págs. 63-64 (0.764)

### `sismo`

**Mide:** Una sección dividida en varios fragmentos

**Pregunta:** ¿Qué debe hacer el titular ante un sismo?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> Ante un sismo, el titular debe implementar diversas medidas preventivas como diseñar instalaciones bajo criterios sismo-resistentes, capacitar al personal, difundir y mantener despejadas las áreas de seguridad, mantener los vehículos estacionados sin obstruir accesos y en posición de salida, realizar simulacros periódicos e informar a los nuevos trabajadores sobre el Protocolo de Respuesta a Emergencias [1]. Para controlar la emergencia, el personal debe mantener la calma, movilizarse a la zona de seguridad más cercana, comunicarse entre áreas para informar la situación, realizar un recuento de trabajadores y buscar asistencia para los lesionados [1]. Posteriormente, se debe revisar si hay daños materiales o emergencias ambientales por derrame de sustancias peligrosas o combustibles y, de no existir daños colaterales, dar por terminada la emergencia para retornar a las labores [1]. Además, si la emergencia configura un riesgo o potencial afectación a algún componente ambiental, se debe comunicar a la Superintendencia del Medio Ambiente (SMA) en un plazo de 24 horas [2].

1. Considerando 12.1, págs. 93-94 (0.697)
2. Considerando 12.1, pág. 94 (0.657)

### `inversion`

**Mide:** Una cifra dentro de una sección de antecedentes generales

**Pregunta:** ¿Cuál es el monto de inversión del proyecto?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> El monto de inversión estimado para la ejecución del Proyecto será de USD $30 millones de dólares americanos [1].

1. Considerando 4.1, págs. 3-4 (0.720)

### `compromisos-voluntarios`

**Mide:** Una enumeración: hay que recuperar las cuatro subsecciones. Falló en producción al verificar #25

**Pregunta:** ¿Qué compromisos ambientales voluntarios contempla el proyecto?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> El proyecto contempla realizar los compromisos ambientales voluntarios de Monitoreo Calidad del Aire [1], Plan de Monitoreo Calidad de Aguas Subterráneas [2], Plan de Monitoreo Calidad de Aguas Superficiales [4], y Plan de Monitoreo de Ruido [3].

1. Considerando 8.1, pág. 86 (0.783)
2. Considerando 8.2, págs. 86-87 (0.770)
3. Considerando 8.4, pág. 88 (0.762)
4. Considerando 8.3, págs. 87-88 (0.760)

### `compromisos-voluntarios-cuales`

**Mide:** La misma enumeración, redactada como se preguntó en producción al verificar #58: la respuesta agregó la charla de difusión de la 11.1, que es monitoreo participativo y no un compromiso voluntario

**Pregunta:** ¿Cuáles son los compromisos ambientales voluntarios del proyecto?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> Los compromisos ambientales voluntarios del proyecto son el monitoreo de calidad del aire, el plan de monitoreo de calidad de aguas subterráneas, el plan de monitoreo calidad de aguas superficiales y el plan de monitoreo de ruido [1][2][3][4].

1. Considerando 8.1, pág. 86 (0.783)
2. Considerando 8.2, págs. 86-87 (0.767)
3. Considerando 8.4, pág. 88 (0.762)
4. Considerando 8.3, págs. 87-88 (0.758)

### `estanque-agua-tratada`

**Mide:** El documento reconoce su propia incongruencia y fija la exigencia, pero no explica la causa: la respuesta no debe inventarla

**Pregunta:** ¿A qué se deben las incongruencias en el volumen útil del estanque de acumulación de agua tratada mencionadas en el texto, y se encuentra debidamente garantizado que contará con una capacidad mínima de 4.000 litros/día y un tiempo de retención no superior a 48 horas?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> Los pasajes no indican a qué se deben las incongruencias en el volumen del estanque de acumulación de agua tratada, pero señalan que existen incongruencias respecto al volumen señalado entre los antecedentes presentados [1]. Ante esto, el titular deberá garantizar que dicho estanque cuente con un volumen útil mínimo equivalente al caudal de diseño de aguas servidas (4.000 litros/día) para enfrentar eventuales contingencias, y que el tiempo de retención no exceda las 48 horas [1].

1. Considerando 6.2.1, págs. 62-63 (0.617)

### `estanque-tk65`

**Mide:** Premisa falsa: el documento dice 34 m3 en ambas secciones y '4 m3' no aparece. La respuesta no debe confirmar la contradicción

**Pregunta:** ¿Cuál es la capacidad real y definitiva del estanque TK-65 (destinado a xileno/diluyente), dado que en la descripción del numeral 4.3.2 se señalan 34 m3, mientras que en la Tabla 7.5.1 sobre cumplimiento del DS 594 se indica 4 m3?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> Los pasajes señalan que el estanque de almacenamiento denominado TK-65 tiene una capacidad de 34 m3 [1][2][3]. Los pasajes no contienen información sobre una capacidad de 4 m3 ni mencionan la Tabla 7.5.1 sobre cumplimiento del DS 594.

1. Considerando 4.3, págs. 34-35 (0.716)
2. Considerando 7.5, págs. 77-78 (0.703)
3. Considerando 7.5, págs. 76-77 (0.703)

### `fechas-expediente`

**Mide:** Fechas repartidas en tres secciones, con una premisa sesgada: la secuencia es la normal de una evaluación

**Pregunta:** ¿Por qué existe un desfase temporal en la secuencia del expediente de evaluación, figurando un proceso de Participación Ciudadana (PAC) desarrollado entre octubre y noviembre de 2024, pero citando Adendas presentadas en abril y noviembre de 2025 e Informes Consolidados de Evaluación (ICE) de diciembre de 2025?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> Los pasajes no contienen información para explicar por qué existe un desfase temporal en la secuencia del expediente de evaluación, pero señalan que la DIA del proyecto fue publicada en agosto de 2024 [2], se admitió la solicitud de participación ciudadana tras cumplir los requisitos legales [2], y el proceso respectivo se desarrolló entre el 17 de octubre y el 15 de noviembre de 2024 [2]. Asimismo, indican que el ICE y la recomendación de aprobación del Director Regional del SEA de la Región de Coquimbo tienen fecha del 23 de diciembre de 2025 [1][3].

1. Vistos 4, pág. 1 (0.691)
2. Considerando 13, pág. 106 (0.677)
3. Considerando 2, pág. 2 (0.657)

### `limites-genericos`

**Mide:** Un hecho cierto repartido en cuatro secciones; el motivo no está en el documento

**Pregunta:** ¿Por qué en los Planes de Monitoreo de Calidad de Aguas Subterráneas y Superficiales (numerales 8.2, 8.3, 10.2 y 10.3) se omite el valor normativo numérico o la norma específica de referencia, utilizando la fórmula genérica "límites máximos permitidos por la autoridad correspondiente"?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> Los pasajes entregados no contienen información que explique por qué se utiliza la fórmula genérica "límites máximos permitidos por la autoridad correspondiente" en lugar de un valor normativo numérico o una norma específica de referencia en los planes de monitoreo de calidad de aguas subterráneas y superficiales. No obstante, los pasajes señalan que en dichos planes los límites permitidos o indicadores de cumplimiento corresponden precisamente a los límites máximos permitidos por la autoridad correspondiente [1][2][3][4].

1. Considerando 10.2, pág. 90 (0.706)
2. Considerando 10.3, págs. 90-91 (0.705)
3. Considerando 8.3, págs. 87-88 (0.695)
4. Considerando 8.2, págs. 86-87 (0.691)

### `norma-aplicable`

**Mide:** Juicio jurídico que el documento no hace. Trampa: el D.S. 90/2000 aparece, pero para la descarga de riles; la respuesta no debe atribuirlo a los planes de monitoreo

**Pregunta:** ¿Cómo se garantiza la certeza jurídica y la fiscalizabilidad de estos compromisos ante la Superintendencia del Medio Ambiente (SMA) si no se estipula de forma explícita la norma aplicable (ej. NCh 1333 o D.S. N° 90/2000)?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> Los pasajes entregados no contienen información sobre la NCh 1333, el D.S. N° 90/2000 ni sobre la garantía de certeza jurídica ante la Superintendencia del Medio Ambiente sin la estipulación explícita de dichas normas. No obstante, indican que los registros de los indicadores de cumplimiento y los antecedentes estarán disponibles en las oficinas del Proyecto y en plataformas para la revisión y fiscalización de la autoridad competente [1][2][3][4][5].

1. Considerando 7.1, págs. 67-68 (0.709)
2. Considerando 7.2, pág. 71 (0.705)
3. Considerando 7.1, págs. 66-67 (0.703)
4. Considerando 7.1, págs. 69-70 (0.703)
5. Considerando 7.1, págs. 65-66 (0.698)

### `plazos-contingencias`

**Mide:** Premisa en parte falsa: las 24 horas aplican también a condiciones climáticas (12.2). Hay que juntar pasajes de varias contingencias para corregirla

**Pregunta:** ¿Por qué el Plan de Prevención de Contingencias y Emergencias fija un plazo de notificación a la SMA de 24 horas en caso de evento sísmico (numeral 12.1), mientras que para el resto de las contingencias (climáticas, remoción en masa, inundación, incendios, filtraciones, etc.) se establece un plazo de 48 horas?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> Los pasajes indican que en caso de emergencias que configuren un riesgo o potencial afectación a algún componente ambiental, se comunicará a la Superintendencia de Medio Ambiente (SMA) en un plazo de 24 horas [1][3], y también señalan que se informará a la autoridad correspondiente y a la SMA mediante un informe dentro de las 48 horas posteriores al suceso [2][4], pero no explican ni justifican por qué existe una diferencia en los plazos de notificación entre el evento sísmico y otras contingencias.

1. Considerando 12.1, pág. 94 (0.792)
2. Considerando 12.6, pág. 99 (0.714)
3. Considerando 12.2, pág. 95 (0.706)
4. Considerando 12.3, págs. 95-96 (0.702)

### `sin-respuesta-precio-cobre`

**Mide:** Del dominio, pero ausente del documento: no debe recibir citas

**Pregunta:** ¿Qué precio del cobre se consideró en la evaluación económica del proyecto?

**Respuesta** (sin texto redactado):

> Los pasajes recuperados no responden esta pregunta. Se muestran los 5 más relacionados, por si contienen algo útil; verifica en ellos antes de concluir que el documento no trata el tema.

1. Considerando 4.3, págs. 27-28 (0.666)
2. Considerando 4.1, págs. 2-3 (0.659)
3. Considerando 4.3, págs. 24-25 (0.656)
4. Considerando 4.4.2, pág. 40 (0.649)
5. Considerando 4.3, págs. 14-15 (0.649)

### `sin-respuesta-sueldos`

**Mide:** Del dominio, pero ausente del documento: no debe recibir citas

**Pregunta:** ¿Cuál será el sueldo promedio de los trabajadores durante la operación?

**Respuesta** (sin texto redactado):

> Los pasajes recuperados no responden esta pregunta. Se muestran los 5 más relacionados, por si contienen algo útil; verifica en ellos antes de concluir que el documento no trata el tema.

1. Considerando 4.4.4, págs. 40-41 (0.680)
2. Considerando 4.4.2, pág. 40 (0.641)
3. Considerando 4.3.2, pág. 22 (0.623)
4. Considerando 5.3, págs. 56-57 (0.617)
5. Considerando 4.3, pág. 7 (0.614)

### `sin-respuesta-energia-solar`

**Mide:** Del dominio, pero ausente del documento: no debe recibir citas

**Pregunta:** ¿Cuántos paneles solares instalará el proyecto para abastecer la planta?

**Respuesta** (sin texto redactado):

> Los pasajes recuperados no responden esta pregunta. Se muestran los 5 más relacionados, por si contienen algo útil; verifica en ellos antes de concluir que el documento no trata el tema.

1. Considerando 4.3, págs. 14-15 (0.623)
2. Considerando 4.3, págs. 27-28 (0.619)
3. Considerando 4.3, págs. 17-19 (0.619)
4. Considerando 9, pág. 88 (0.617)
5. Considerando 4.3, pág. 7 (0.617)

### `sin-respuesta-ajena`

**Mide:** Ajena al dominio: no debe recibir citas

**Pregunta:** ¿Cuál es la receta del pisco sour?

**Respuesta** (sin sustento):

> No se encontraron pasajes en el corpus que sustenten una respuesta a esta pregunta.


