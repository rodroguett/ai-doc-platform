# Evaluación de la búsqueda

- **Documento `rca`:** internal/rag/testdata/rca-tres-valles.pdf
- **Documento `icsara`:** internal/rag/testdata/icsara-tres-valles.pdf
- **Documento `icsara2`:** internal/rag/testdata/icsara-complementario-tres-valles.pdf
- **Fecha:** 2026-10-18
- **Modelo de embeddings:** `gemini-embedding-001`
- **Puntaje mínimo para citar:** 0.60
- **Preguntas:** 26 (22 respondibles, 4 que el documento no responde)

Cada pregunta mide un comportamiento distinto. Las de origen 'autor' las escribió quien construyó el sistema, conociendo el documento: están sesgadas hacia lo que el sistema hace bien. Las de origen 'asistente' las generó un asistente de IA a pedido del dueño del proyecto, y quedaron las que aportaban; algunas traen premisas falsas, a propósito o no.

## Resultado

| Medida | Valor |
|---|---|
| Preguntas con todo lo esperado entre los primeros 5 pasajes | 19 de 22 |
| Ídem, y además con puntaje suficiente para citarse | 19 de 22 |
| Grupos de secciones esperadas recuperados entre los primeros 5 | 38 de 41 |
| Ídem entre los primeros 10 | 40 de 41 |
| Preguntas sin respuesta en el documento que recibirían citas | 3 de 4 |
| Respuestas que citan una sección que no corresponde | 0 de 2 |
| Respuestas redactadas con cifras sin respaldo en sus citas | 0 de 22 |

## Por pregunta

| Pregunta | Origen | Esperado | Recuperado (primeros 5) | Resultado |
|---|---|---|---|---|
| `aire-parametros` | autor | Considerando 10.1 | Considerando 10.1 (0.725)<br>Considerando 8.1 (0.680)<br>Considerando 10.2 (0.665)<br>Considerando 10.3 (0.664)<br>Considerando 7.1 (0.654) | 1/1 grupos en los primeros 5 (1/1 en 10), 1 citables |
| `informes-frecuencia` | autor | Considerando 10.1 o Considerando 8.1 | Considerando 10.1 (0.710)<br>Considerando 10.2 (0.672)<br>Considerando 15 (0.672)<br>Considerando 8.1 (0.667)<br>Considerando 7.1 (0.666) | 1/1 grupos en los primeros 5 (1/1 en 10), 1 citables |
| `vida-util` | autor | Considerando 4.3.3 | Considerando 4.3.3 (0.794)<br>Considerando 4.4.4 (0.732)<br>Considerando 4.3 (0.724)<br>Considerando 4.4.2 (0.717)<br>icsara2:Capítulo II 1.9 (0.742) | 1/1 grupos en los primeros 5 (1/1 en 10), 1 citables |
| `permisos-sectoriales` | autor | Considerando 6.1; Considerando 6.2.1; Considerando 6.2.2 | Considerando 6.2 (0.815)<br>Considerando 6.2.1 (0.813)<br>Considerando 6.1 (0.809)<br>Considerando 6.2.1 (0.789)<br>Considerando 6.2.2 (0.788) | 3/3 grupos en los primeros 5 (3/3 en 10), 3 citables |
| `permisos-sectoriales-cuales` | autor | Considerando 6.1; Considerando 6.2.1; Considerando 6.2.2 | Considerando 6.2 (0.790)<br>Considerando 6.1 (0.780)<br>Considerando 6.2.1 (0.775)<br>Considerando 6.2.1 (0.771)<br>Considerando 6.2.2 (0.764) | 3/3 grupos en los primeros 5 (3/3 en 10), 3 citables |
| `sismo` | autor | Considerando 12.1 | Considerando 12.1 (0.697)<br>Considerando 12.1 (0.657)<br>Considerando 12.3 (0.649)<br>Considerando 20 (0.611)<br>Considerando 12.6 (0.610) | 1/1 grupos en los primeros 5 (1/1 en 10), 1 citables |
| `inversion` | autor | Considerando 4.1 | Considerando 4.1 (0.720)<br>Considerando 9 (0.664)<br>Considerando 4.4.1 (0.663)<br>Considerando 4.4.4 (0.661)<br>Considerando 4.4.2 (0.656) | 1/1 grupos en los primeros 5 (1/1 en 10), 1 citables |
| `compromisos-voluntarios` | autor | Considerando 8.1; Considerando 8.2; Considerando 8.3; Considerando 8.4 | Considerando 8.1 (0.783)<br>Considerando 8.2 (0.770)<br>Considerando 8.4 (0.762)<br>Considerando 8.3 (0.760)<br>icsara:Capítulo VI 1 (0.789) | 4/4 grupos en los primeros 5 (4/4 en 10), 4 citables |
| `compromisos-voluntarios-cuales` | autor | Considerando 8.1; Considerando 8.2; Considerando 8.3; Considerando 8.4 | Considerando 8.1 (0.783)<br>Considerando 8.2 (0.767)<br>Considerando 8.4 (0.762)<br>Considerando 8.3 (0.758)<br>icsara:Capítulo VI 1 (0.779) | 4/4 grupos en los primeros 5 (4/4 en 10), 4 citables |
| `estanque-agua-tratada` | asistente | Considerando 6.2.1 | icsara2:Capítulo III 1.1 (0.666)<br>icsara2:Capítulo II 7.1 (0.666)<br>icsara:Capítulo III 3.5 (0.663)<br>Considerando 4.3 (0.625)<br>icsara2:Capítulo II 7.5 (0.655) | 0/1 grupos en los primeros 5 (1/1 en 10), 0 citables |
| `estanque-tk65` | asistente | Considerando 4.3 o Considerando 7.5 | icsara2:Capítulo II 7.1 (0.749)<br>Considerando 4.3 (0.716)<br>icsara2:Capítulo II 7.5 (0.745)<br>Considerando 7.5 (0.703)<br>Considerando 7.5 (0.703) | 1/1 grupos en los primeros 5 (1/1 en 10), 1 citables |
| `fechas-expediente` | asistente | Vistos 1; Considerando 2 o Considerando 3; Considerando 13 o Considerando 14 | Vistos 4 (0.691)<br>Considerando 14 (0.689)<br>Considerando 13 (0.677)<br>Vistos 2 (0.673)<br>Considerando 2 (0.657) | 2/3 grupos en los primeros 5 (3/3 en 10), 2 citables |
| `limites-genericos` | asistente | Considerando 8.2; Considerando 8.3; Considerando 10.2; Considerando 10.3 | Considerando 10.2 (0.706)<br>Considerando 10.3 (0.705)<br>Considerando 8.3 (0.695)<br>Considerando 8.2 (0.691)<br>icsara:Capítulo VI 3.3 (0.697) | 4/4 grupos en los primeros 5 (4/4 en 10), 4 citables |
| `norma-aplicable` | asistente | Considerando 8.2 o Considerando 8.3 o Considerando 10.2 o Considerando 10.3 | Considerando 7.1 (0.709)<br>Considerando 7.2 (0.705)<br>Considerando 7.1 (0.703)<br>Considerando 7.1 (0.703)<br>Considerando 7.1 (0.698) | 0/1 grupos en los primeros 5 (0/1 en 10), 0 citables |
| `plazos-contingencias` | asistente | Considerando 12.1; Considerando 12.2; Considerando 12.3 o Considerando 12.4 o Considerando 12.5 o Considerando 12.6 o Considerando 12.7 o Considerando 12.8 o Considerando 12.9 | Considerando 12.1 (0.792)<br>Considerando 12.1 (0.734)<br>Considerando 12.6 (0.714)<br>Considerando 12.2 (0.706)<br>Considerando 12.3 (0.702) | 3/3 grupos en los primeros 5 (3/3 en 10), 3 citables |
| `cifra-falsa-informes` | autor | Considerando 10.1 o Considerando 8.1 | Considerando 10.1 (0.659)<br>Considerando 8.1 (0.629)<br>Considerando 10.2 (0.613)<br>Considerando 10.3 (0.612)<br>Considerando 7.1 (0.605) | 1/1 grupos en los primeros 5 (1/1 en 10), 1 citables |
| `cifra-falsa-mp10` | autor | Considerando 10.1 | Considerando 10.1 (0.686)<br>Considerando 8.1 (0.640)<br>icsara2:Capítulo II 2.10 (0.665)<br>Considerando 7.1 (0.632)<br>Considerando 7.1 (0.632) | 1/1 grupos en los primeros 5 (1/1 en 10), 1 citables |
| `cifra-falsa-vida-util` | autor | Considerando 4.3.3 | Considerando 4.3.3 (0.738)<br>Considerando 4.3 (0.717)<br>Considerando 4.4.2 (0.705)<br>Considerando 4.3 (0.699)<br>Considerando 4.4.4 (0.699) | 1/1 grupos en los primeros 5 (1/1 en 10), 1 citables |
| `cifra-falsa-inversion` | autor | Considerando 4.1 | Considerando 4.1 (0.679)<br>Considerando 4.3 (0.655)<br>Considerando 4.3 (0.647)<br>Considerando 4.3 (0.646)<br>Considerando 4.3 (0.645) | 1/1 grupos en los primeros 5 (1/1 en 10), 1 citables |
| `entre-documentos-contingencia` | autor | icsara:Capítulo V 1.11; icsara2:Capítulo V 1.3 | Considerando 20 (0.709)<br>icsara:Capítulo V 1.6 (0.738)<br>icsara2:Capítulo V 1.3 (0.735)<br>icsara:Capítulo V 1.11 (0.726)<br>icsara:Capítulo VI 4.4 (0.718) | 2/2 grupos en los primeros 5 (2/2 en 10), 2 citables |
| `entre-documentos-mano-de-obra` | autor | icsara:Capítulo I 4.2; Considerando 4.4.4 | Considerando 4.4.4 (0.694)<br>icsara:Capítulo I 4.2 (0.715)<br>Considerando 14 (0.675)<br>Resuelvo 6 (0.654)<br>Considerando 7.12 (0.648) | 2/2 grupos en los primeros 5 (2/2 en 10), 2 citables |
| `entre-documentos-afloramiento` | autor | icsara:Capítulo V 1.5 | icsara:Capítulo V 1.5 (0.759)<br>Considerando 15 (0.717)<br>Considerando 20 (0.711)<br>Considerando 12.1 (0.708)<br>Considerando 16 (0.696) | 1/1 grupos en los primeros 5 (1/1 en 10), 1 citables |
| `sin-respuesta-precio-cobre` | autor | — | Considerando 4.3 (0.666)<br>Considerando 4.1 (0.659)<br>Considerando 4.3 (0.656)<br>Considerando 4.4.2 (0.649)<br>Considerando 4.3 (0.649) | no respondible: primer puntaje 0.666, CITARÍA |
| `sin-respuesta-sueldos` | autor | — | Considerando 4.4.4 (0.680)<br>icsara:Capítulo I 4.2 (0.685)<br>Considerando 4.4.2 (0.641)<br>Considerando 4.3.2 (0.623)<br>Considerando 5.3 (0.617) | no respondible: primer puntaje 0.680, CITARÍA |
| `sin-respuesta-energia-solar` | autor | — | Considerando 4.3 (0.623)<br>Considerando 4.3 (0.619)<br>Considerando 4.3 (0.619)<br>Considerando 9 (0.617)<br>Considerando 4.3 (0.617) | no respondible: primer puntaje 0.623, CITARÍA |
| `sin-respuesta-ajena` | autor | — | Resuelvo 6 (0.542)<br>Considerando 4.3 (0.540)<br>Considerando 4.3 (0.539)<br>Considerando 4.3 (0.538)<br>Considerando 23 (0.533) | no respondible: primer puntaje 0.542, sin citas |

## Puntaje mínimo

Para cada umbral: cuántas preguntas respondibles tendrían su mejor pasaje esperado en condiciones de citarse, y cuántas sin respuesta recibirían citas.

| Umbral | Respondibles con sustento citable | Sin respuesta que recibirían citas |
|---|---|---|
| 0.500 | 20 de 22 | 4 de 4 |
| 0.525 | 20 de 22 | 4 de 4 |
| 0.550 | 20 de 22 | 3 de 4 |
| 0.575 | 20 de 22 | 3 de 4 |
| 0.600 **(actual)** | 20 de 22 | 3 de 4 |
| 0.625 | 20 de 22 | 2 de 4 |
| 0.650 | 20 de 22 | 2 de 4 |
| 0.675 | 19 de 22 | 1 de 4 |
| 0.700 | 15 de 22 | 0 de 4 |
| 0.725 | 9 de 22 | 0 de 4 |
| 0.750 | 7 de 22 | 0 de 4 |

## Respuestas redactadas, para revisión manual

La verificación automática comprueba que las citas existan, no que la respuesta sea correcta. Estas respuestas se revisan a mano contra lo que cada pregunta mide.

### `aire-parametros`

**Mide:** Un dato con varias partes en una sola sección

**Pregunta:** ¿Qué parámetros de calidad del aire se monitorean y con qué límites?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> Para caracterizar el estado y evolución de la calidad del aire se monitorean los parámetros de Material Particulado Respirable (MP-10), Material Particulado Sedimentable (MPS) y parámetros meteorológicos [1][2]. Los límites comprometidos para el Material Particulado Respirable MP-10 corresponden a un valor máximo promedio diario de 150 ug/m3N y un valor promedio trianual de 50 ug/m3N [1]. Para el Material Particulado Sedimentable (MPS), el límite se rige por la norma de referencia establecida en el Decreto Supremo N°4/1992 del Ministerio de Salud, mientras que para los parámetros meteorológicos (velocidad y dirección del viento, temperatura, humedad relativa y pluviometría) no se especifican límites numéricos en los pasajes, sino la medición continua de dichas variables [1].

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

> El Proyecto contempla una vida útil de 11 años de operación [1]. Por su parte, la fase de cierre tiene una duración estimada de tres años [1][2][3].

1. Considerando 4.3.3, pág. 35 (0.794)
2. Considerando 4.3, pág. 36 (0.724)
3. Capítulo II 1.9, pág. 6 (0.742)

### `permisos-sectoriales`

**Mide:** Una respuesta que combina subsecciones hermanas: cada permiso está en la suya

**Pregunta:** ¿Qué permisos ambientales sectoriales aplican al proyecto?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> Al proyecto le aplican los permisos ambientales sectoriales mixtos [1], específicamente el permiso para la construcción, reparación, modificación y ampliación de cualquier obra pública o particular destinada a la evacuación, tratamiento o disposición final de desagües y aguas servidas (establecido en el artículo 138 del Reglamento del SEIA) [3] y el permiso para subdividir y urbanizar terrenos rurales o para construcciones fuera de los límites urbanos (establecido en el artículo 160 del Reglamento del SEIA) [4]. El proyecto no contempla permisos ambientales sectoriales de contenido únicamente ambiental [2].

1. Considerando 6.2, pág. 61 (0.815)
2. Considerando 6.1, pág. 61 (0.809)
3. Considerando 6.2.1, págs. 62-63 (0.789)
4. Considerando 6.2.2, págs. 63-64 (0.788)

### `permisos-sectoriales-cuales`

**Mide:** La misma enumeración, con la sigla con que la escribiría alguien del sector. Sin expandir la sigla, la búsqueda dejaba fuera la 6.2.2 y la respuesta omitía el permiso del artículo 160 (#62)

**Pregunta:** ¿Cuáles son los PAS del proyecto?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> Los permisos ambientales sectoriales aplicables al proyecto corresponden a los permisos ambientales sectoriales mixtos [1], ya que el proyecto no contempla permisos ambientales sectoriales de contenido únicamente ambiental [2]. Entre los permisos mixtos se encuentran el permiso para la construcción, reparación, modificación y ampliación de cualquier obra pública o particular destinada a la evacuación, tratamiento o disposición final de desagües y aguas servidas (establecido en el artículo 138 del Reglamento del SEIA) [3], y el permiso para subdividir y urbanizar terrenos rurales o para construcciones fuera de los límites urbanos (establecido en el artículo 160 del Reglamento del SEIA) [4].

1. Considerando 6.2, pág. 61 (0.790)
2. Considerando 6.1, pág. 61 (0.780)
3. Considerando 6.2.1, págs. 62-63 (0.771)
4. Considerando 6.2.2, págs. 63-64 (0.764)

### `sismo`

**Mide:** Una sección dividida en varios fragmentos

**Pregunta:** ¿Qué debe hacer el titular ante un sismo?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> Ante un sismo, el titular debe implementar medidas preventivas como diseñar instalaciones bajo criterios sismo-resistentess, capacitar al personal, difundir y mantener despejadas áreas de seguridad libres de elementos de riesgo, mantener los vehículos estacionados sin obstruir accesos y en posición de salida, realizar simulacros periódicos e informar a los nuevos trabajadores sobre el protocolo de respuesta a emergencias [1]. Para controlar la emergencia, el personal debe mantener la calma mediante el llamado de un encargado, movilizarse a la zona de seguridad más cercana, comunicarse entre áreas para conocer la situación, realizar un recuento de trabajadores y asistir a los lesionados [1]. Posteriormente, se debe revisar si existen daños materiales y posibles emergencias ambientales por derrames, y de no haber daños colaterales, se dará por terminada la emergencia para retornar a las labores [1]. Además, si la emergencia configura un riesgo o potencial afectación a algún componente ambiental, se debe comunicar a la Superintendencia del Medio Ambiente en un plazo de 24 horas [2].

1. Considerando 12.1, págs. 93-94 (0.697)
2. Considerando 12.1, pág. 94 (0.657)

### `inversion`

**Mide:** Una cifra dentro de una sección de antecedentes generales

**Pregunta:** ¿Cuál es el monto de inversión del proyecto?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> El monto de inversión estimado para la ejecución del proyecto será de 30 millones de dólares americanos [1].

1. Considerando 4.1, págs. 3-4 (0.720)

### `compromisos-voluntarios`

**Mide:** Una enumeración: hay que recuperar las cuatro subsecciones. Falló en producción al verificar #25

**Pregunta:** ¿Qué compromisos ambientales voluntarios contempla el proyecto?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> El proyecto contempla realizar como compromisos ambientales voluntarios el monitoreo de la calidad del aire [1], el plan de monitoreo de la calidad de aguas subterráneas [2], el plan de monitoreo de ruido [3] y el plan de monitoreo de la calidad de aguas superficiales [4].

1. Considerando 8.1, pág. 86 (0.783)
2. Considerando 8.2, págs. 86-87 (0.770)
3. Considerando 8.4, pág. 88 (0.762)
4. Considerando 8.3, págs. 87-88 (0.760)

### `compromisos-voluntarios-cuales`

**Mide:** La misma enumeración, redactada como se preguntó en producción al verificar #58: la respuesta agregó la charla de difusión de la 11.1, que es monitoreo participativo y no un compromiso voluntario

**Pregunta:** ¿Cuáles son los compromisos ambientales voluntarios del proyecto?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> El proyecto contempla realizar como compromisos ambientales voluntarios el monitoreo de la calidad del aire [1], el plan de monitoreo de la calidad de aguas subterráneas [2], el plan de monitoreo de calidad de aguas superficiales [4] y el plan de monitoreo de ruido [3].

1. Considerando 8.1, pág. 86 (0.783)
2. Considerando 8.2, págs. 86-87 (0.767)
3. Considerando 8.4, pág. 88 (0.762)
4. Considerando 8.3, págs. 87-88 (0.758)

### `estanque-agua-tratada`

**Mide:** El documento reconoce su propia incongruencia y fija la exigencia, pero no explica la causa: la respuesta no debe inventarla

**Pregunta:** ¿A qué se deben las incongruencias en el volumen útil del estanque de acumulación de agua tratada mencionadas en el texto, y se encuentra debidamente garantizado que contará con una capacidad mínima de 4.000 litros/día y un tiempo de retención no superior a 48 horas?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> Los pasajes no mencionan ninguna incongruencia en el volumen de un estanque de acumulación de agua tratada ni hacen referencia a una capacidad mínima de 4.000 litros/día ni a un tiempo de retención no superior a 48 horas. Respecto al volumen del sistema de tratamiento de aguas servidas, se señala una diferencia de 80 litros entre el volumen útil requerido de 7.080 litros según los cálculos y la fosa séptica instalada, que tiene un tamaño de 7.900 litros (7.000 litros útiles) [1].

1. Capítulo III 1.1, págs. 16-17 (0.666)

### `estanque-tk65`

**Mide:** Premisa falsa: el documento dice 34 m3 en ambas secciones y '4 m3' no aparece. La respuesta no debe confirmar la contradicción

**Pregunta:** ¿Cuál es la capacidad real y definitiva del estanque TK-65 (destinado a xileno/diluyente), dado que en la descripción del numeral 4.3.2 se señalan 34 m3, mientras que en la Tabla 7.5.1 sobre cumplimiento del DS 594 se indica 4 m3?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> Los pasajes señalan que el estanque de almacenamiento denominado TK-65 tiene una capacidad de 34 m3 [1][2][3], sin mencionar un valor de 4 m3 ni hacer referencia a un numeral 4.3.2.

1. Considerando 4.3, págs. 34-35 (0.716)
2. Considerando 7.5, págs. 77-78 (0.703)
3. Considerando 7.5, págs. 76-77 (0.703)

### `fechas-expediente`

**Mide:** Fechas repartidas en tres secciones, con una premisa sesgada: la secuencia es la normal de una evaluación

**Pregunta:** ¿Por qué existe un desfase temporal en la secuencia del expediente de evaluación, figurando un proceso de Participación Ciudadana (PAC) desarrollado entre octubre y noviembre de 2024, pero citando Adendas presentadas en abril y noviembre de 2025 e Informes Consolidados de Evaluación (ICE) de diciembre de 2025?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> Los pasajes no contienen información para explicar por qué existe un desfase temporal en la secuencia del expediente de evaluación, pero señalan que la DIA del proyecto fue publicada en agosto de 2024, el proceso de participación ciudadana se desarrolló entre el 17 de octubre y el 15 de noviembre de 2024, y tanto el ICE como el ICE N°20250410986 datan del 23 de diciembre de 2025 [1][2].

1. Vistos 4, pág. 1 (0.691)
2. Considerando 13, pág. 106 (0.677)

### `limites-genericos`

**Mide:** Un hecho cierto repartido en cuatro secciones; el motivo no está en el documento

**Pregunta:** ¿Por qué en los Planes de Monitoreo de Calidad de Aguas Subterráneas y Superficiales (numerales 8.2, 8.3, 10.2 y 10.3) se omite el valor normativo numérico o la norma específica de referencia, utilizando la fórmula genérica "límites máximos permitidos por la autoridad correspondiente"?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> Los pasajes no contienen información que explique por qué se utiliza la fórmula genérica "límites máximos permitidos por la autoridad correspondiente" en lugar de un valor normativo numérico o una norma específica de referencia en dichos planes de monitoreo. Lo que los pasajes señalan es que tanto en los planes de seguimiento ambiental como en los compromisos ambientales voluntarios de calidad de aguas subterráneas y superficiales, los límites permitidos, comprometidos o indicadores de cumplimiento corresponden precisamente a los límites máximos permitidos por la autoridad correspondiente [1][2][3][4].

1. Considerando 10.2, pág. 90 (0.706)
2. Considerando 10.3, págs. 90-91 (0.705)
3. Considerando 8.3, págs. 87-88 (0.695)
4. Considerando 8.2, págs. 86-87 (0.691)

### `norma-aplicable`

**Mide:** Juicio jurídico que el documento no hace. Trampa: el D.S. 90/2000 aparece, pero para la descarga de riles; la respuesta no debe atribuirlo a los planes de monitoreo

**Pregunta:** ¿Cómo se garantiza la certeza jurídica y la fiscalizabilidad de estos compromisos ante la Superintendencia del Medio Ambiente (SMA) si no se estipula de forma explícita la norma aplicable (ej. NCh 1333 o D.S. N° 90/2000)?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> Los pasajes entregados no contienen información sobre la NCh 1333, el D.S. N° 90/2000 ni sobre la garantía de certeza jurídica ante la Superintendencia del Medio Ambiente (SMA). No obstante, indican que los registros de los indicadores de cumplimiento de las medidas y la normativa ambiental aplicable estarán disponibles en las oficinas del Proyecto o dispuestos en plataformas para la revisión y fiscalización por parte de la autoridad competente [1][2][3][4][5].

1. Considerando 7.1, págs. 67-68 (0.709)
2. Considerando 7.2, pág. 71 (0.705)
3. Considerando 7.1, págs. 66-67 (0.703)
4. Considerando 7.1, págs. 69-70 (0.703)
5. Considerando 7.1, págs. 65-66 (0.698)

### `plazos-contingencias`

**Mide:** Premisa en parte falsa: las 24 horas aplican también a condiciones climáticas (12.2). Hay que juntar pasajes de varias contingencias para corregirla

**Pregunta:** ¿Por qué el Plan de Prevención de Contingencias y Emergencias fija un plazo de notificación a la SMA de 24 horas en caso de evento sísmico (numeral 12.1), mientras que para el resto de las contingencias (climáticas, remoción en masa, inundación, incendios, filtraciones, etc.) se establece un plazo de 48 horas?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> Los pasajes entregados no contienen información sobre los motivos o razones por los cuales se establecen plazos de notificación de 24 horas para el caso de evento sísmico y de 48 horas para otras contingencias. Los pasajes señalan que en caso de emergencias que configuren un riesgo o potencial afectación a algún componente ambiental se comunicará a la SMA dentro de un plazo de 24 horas [1][3], y que se informará a la autoridad correspondiente y a la SMA mediante un informe dentro de las 48 horas posteriores al suceso [2][4].

1. Considerando 12.1, pág. 94 (0.792)
2. Considerando 12.6, pág. 99 (0.714)
3. Considerando 12.2, pág. 95 (0.706)
4. Considerando 12.3, págs. 95-96 (0.702)

### `cifra-falsa-informes`

**Mide:** Premisa numérica falsa: son cada 6 meses. La respuesta no debe afirmar los 3 meses; si los afirma citando, la verificación de cifras debe avisar (#76)

**Pregunta:** ¿Por qué los informes de calidad del aire se entregan cada 3 meses?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> Los pasajes señalan que la entrega de los informes de seguimiento a la autoridad competente se realiza cada 6 meses, no cada 3 meses [1][2].

1. Considerando 10.1, págs. 89-90 (0.659)
2. Considerando 8.1, pág. 86 (0.629)

### `cifra-falsa-mp10`

**Mide:** Premisa numérica falsa: el límite diario es 150 ug/m3N (#76)

**Pregunta:** ¿Por qué la RCA fija el límite diario de MP-10 en 100 ug/m3N?

**Respuesta** (redactada por `gemini-3.5-flash-lite`):

> Los pasajes no contienen información sobre por qué se fija el límite diario de MP-10 en 100 ug/m3N ni mencionan ese valor, ya que respecto al material particulado respirable MP-10 solo establecen un valor máximo promedio diario de 150 ug/m3N y un valor promedio trianual de 50 ug/m3N [1].

1. Considerando 10.1, págs. 89-90 (0.686)

### `cifra-falsa-vida-util`

**Mide:** Premisa numérica falsa: la vida útil es de 11 años, seguida de tres de cierre (#76)

**Pregunta:** ¿Qué se hará al terminar los 15 años de operación del proyecto?

**Respuesta** (redactada por `gemini-3.1-flash-lite`):

> Los pasajes no indican qué se hará al terminar 15 años de operación, ya que establecen que la fase de operación tiene una duración de 11 años, con inicio en enero de 2027 y término en diciembre de 2037 [3][5]. Tras finalizar la operación, el proyecto contempla una fase de cierre con una duración estimada de tres años, cuyo propósito es ejecutar medidas para prevenir, minimizar o controlar riesgos o efectos negativos sobre la salud, seguridad de las personas y el medio ambiente [1][2]. Durante esta etapa se realizarán actividades como el desmantelamiento, desenergización y retiro de infraestructura, protección de estructuras remanentes, cierre de accesos y señalizaciones, estabilización de taludes, sellado de bocaminas y piques, manejo de soluciones remanentes, retiro de materiales, escombros y repuestos, cierre de almacenes de explosivos, actividades de perfilamiento y compactación, restauración del área intervenida, prevención de futuras emisiones, y labores de mantención, conservación y supervisión [2][4].

1. Considerando 4.3.3, pág. 35 (0.738)
2. Considerando 4.3, pág. 36 (0.717)
3. Considerando 4.4.2, pág. 40 (0.705)
4. Considerando 4.3, págs. 36-37 (0.699)
5. Considerando 4.4.4, págs. 40-41 (0.699)

### `cifra-falsa-inversion`

**Mide:** Premisa numérica falsa: la inversión es de USD 30 millones, y el documento no dice cómo se distribuye (#76)

**Pregunta:** ¿Cómo se distribuye la inversión de USD 300 millones del proyecto?

**Respuesta** (redactada por `gemini-3.1-flash-lite`):

> Los pasajes entregados no mencionan una inversión de USD 300 millones, sino que indican que la inversión estimada para la ejecución del proyecto es de USD 30 millones [1].

1. Considerando 4.1, págs. 3-4 (0.679)

### `entre-documentos-contingencia`

**Mide:** Un hecho que solo está en los ICSARA, y que cada uno trata a su modo: el primero pide relacionarlo con el proyecto, el complementario aclarar si las medidas ya estaban operativas (#83)

**Pregunta:** ¿Qué contingencia ambiental ocurrió en junio de 2024 y qué le pidió el SEA al titular al respecto?

**Respuesta** (redactada por `gemini-3.1-flash-lite`):

> En junio de 2024 ocurrió un evento de precipitaciones que provocó el deslizamiento o derrame de material lixiviado hacia la quebrada de Quilmenco, afluente del Río Choapa [1][2][3]. Ante esto, el SEA solicitó al titular aclarar si las medidas presentadas para enfrentar dicha contingencia ya estaban implementadas o si se construirán, advirtiendo que, de haber estado operativas, deben ser replanteadas para asegurar su efectividad ante eventos de lluvia [2]. Asimismo, se pidió al titular relacionar este evento con el proyecto y complementar el Plan de Prevención de Contingencias y Emergencias describiendo acciones para minimizar riesgos [3], además de evaluar la incorporación de un punto extra de monitoreo en un canal de regadío en el sector de Cárcamo [4]. Finalmente, se exigió incorporar medidas específicas para accidentes que comprometan recursos hídricos, las cuales incluyen informar a la Superintendencia del Medio Ambiente antes de 24 horas detallando la descripción del accidente, las acciones de control, la evaluación de efectos sobre los recursos hídricos y, de ser necesario, un programa de descontaminación [1].

1. Capítulo V 1.6, pág. 38 (0.738)
2. Capítulo V 1.3, pág. 38 (0.735)
3. Capítulo V 1.11, pág. 40 (0.726)
4. Capítulo VI 4.4, pág. 41 (0.718)

### `entre-documentos-mano-de-obra`

**Mide:** Una solicitud del ICSARA y su resultado en la RCA: la respuesta debe atribuir cada parte a su documento (#83)

**Pregunta:** ¿Qué pidió el SEA sobre la contratación de mano de obra local y cómo quedó en la RCA?

**Respuesta** (redactada por `gemini-3.1-flash-lite`):

> El Servicio de Evaluación Ambiental solicitó al titular privilegiar la contratación de mano de obra local de las comunas de Illapel y Salamanca mediante la coordinación con las Oficinas Municipales de Inserción Laboral (OMIL), además de requerir que se indicara el número de personas contempladas bajo esta modalidad [2]. En la Resolución de Calificación Ambiental (RCA), se estableció que se coordinará con las OMIL de dichas comunas para publicar vacantes laborales, buscando así priorizar la contratación de mano de obra local [1].

1. Considerando 4.4.4, págs. 40-41 (0.694)
2. Capítulo I 4.2, pág. 5 (0.715)

### `entre-documentos-afloramiento`

**Mide:** El ICSARA pidió esa medida, pero la RCA no la menciona: la respuesta no debe atribuirla a la RCA (#83)

**Pregunta:** ¿La RCA exige avisar a la SMA en menos de 24 horas ante un afloramiento de aguas, como pidió el SEA?

**Respuesta** (redactada por `gemini-3.1-flash-lite`):

> Los pasajes entregados no contienen información que confirme si la Resolución de Calificación Ambiental (RCA) exige específicamente el aviso en menos de 24 horas ante un afloramiento de aguas, ya que dicha instrucción proviene de un documento de solicitud de aclaración (ICSARA). No obstante, los pasajes sí establecen que, ante emergencias que impliquen riesgo o afectación a componentes ambientales, el titular debe comunicar a la Superintendencia del Medio Ambiente (SMA) en un plazo de 24 horas [1].

1. Considerando 12.1, pág. 94 (0.708)

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
2. Capítulo I 4.2, pág. 5 (0.685)
3. Considerando 4.4.2, pág. 40 (0.641)
4. Considerando 4.3.2, pág. 22 (0.623)
5. Considerando 5.3, págs. 56-57 (0.617)

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


