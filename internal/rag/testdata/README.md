# Documentos de prueba

## `rca-tres-valles.pdf`

Resolución de Calificación Ambiental del proyecto "Continuidad Operacional
Minera Tres Valles", de la Comisión de Evaluación de la Región de Coquimbo,
firmada el 13 de enero de 2026.

- **Origen:** Servicio de Evaluación Ambiental,
  <https://firma.sea.gob.cl/publicaciones/2026/01/14/1768398297_2167574355>
- **Descargado:** 2026-10-11
- **sha256:** `0eaae374a53b0d10b103f9dad51ef6e9578a0da995fa495fa04b44d3e2cc2fac`
- **Páginas:** 108

Es un documento público, sin modificar. Se versiona completo y no un
extracto: separar páginas de un PDF firmado produce un archivo más grande
que el original, y volver a renderizarlo elimina justamente lo que lo hace
representativo, el sello de firma incrustado en cada página que la primera
librería evaluada no pudo leer (ADR-0009).

El PDF publicado no trae el número ni la fecha de la resolución: en su
lugar figuran los marcadores `<NUM_RES>` y `<FECHA_RESOLUCION>`.

Los tests dependen de hechos de este documento: tiene 7 vistos, 23
considerandos y 6 resuelvos, y el plan de seguimiento de calidad del aire
está en el considerando 10.1, páginas 89 y 90.

## `icsara-tres-valles.pdf` e `icsara-complementario-tres-valles.pdf`

Los dos Informes Consolidados de Solicitud de Aclaraciones, Rectificaciones
y/o Ampliaciones (ICSARA) que el SEA emitió durante la evaluación del mismo
proyecto: las preguntas que el titular respondió en sus dos adendas, antes
de la RCA.

| | ICSARA | ICSARA complementario |
|---|---|---|
| Fecha | 14 de agosto de 2024 | 5 de junio de 2025 |
| Origen | <https://firma.sea.gob.cl/publicaciones/2024/08/14/1723644341_2162885991> | <https://firma.sea.gob.cl/publicaciones/2025/06/05/1749154621_2165463581> |
| Descargado | 2026-10-18 | 2026-10-18 |
| sha256 | `b0b8a7dc3563a697c91b432c49d127524d54396a2e5ffed782789069ef57fcd2` | `73f010be4be893bd02301338bbcd91dc4d420c351a6a2abed411af1a81e84764` |
| Páginas | 45 | 43 |
| Capítulos | 12 (I a XII) | 10 (I a X) |

Son documentos públicos, sin modificar. Como la RCA, traen marcadores en
lugar del número y la fecha del informe (`<NUM_ICSARA>`,
`<CIUDAD_FECHA_INFORME>`).

Los tests dependen de hechos de estos documentos: sus capítulos con
numeración romana, y que la solicitud 1.3 del capítulo V del primero, en la
página 37, trata de los riesgos del plan de contingencias.
