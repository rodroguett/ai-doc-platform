# Evaluación

El conjunto [`eval/rca-tres-valles.json`](../../eval/rca-tres-valles.json) tiene 17 preguntas sobre la RCA de prueba, cada una con las secciones que la responden, o sin ninguna si el documento no la responde. Cada pregunta mide un comportamiento distinto; no hay preguntas de relleno.

| Origen | Preguntas | Sesgo |
|---|---|---|
| `autor` | 11: siete verificadas a mano en producción y cuatro que el documento no responde | Las escribió quien construyó el sistema, conociendo el documento |
| `asistente` | 6 de revisión crítica: contradicciones, juicios jurídicos, premisas falsas | Las generó un asistente de IA a pedido del dueño del proyecto, que eligió las que aportaban |

## Cómo se corre

```bash
make db-up
docker exec aidoc-pg createdb -U postgres aidoc_eval   # una vez
GEMINI_API_KEY=... make eval
```

Consume cuota del proveedor de embeddings, y con `-generate` también de generación, así que no corre en CI ni en los tests. La primera vez ingiere el documento en una base aparte (unos 112.000 tokens, unos cuatro minutos); las siguientes lo reutilizan. El informe queda en [`rca-tres-valles.md`](rca-tres-valles.md).

## Resultado del 2026-10-14

### Búsqueda

| Medida | Valor |
|---|---|
| Preguntas con todo lo esperado entre los primeros 5 pasajes | 10 de 13 |
| Grupos de secciones esperadas entre los primeros 5 | 20 de 24 |
| Ídem entre los primeros 10 | 23 de 24 |
| Preguntas sin respuesta en el documento que recibirían citas | **3 de 4** |

Las tres preguntas que no lo logran: los **compromisos voluntarios** (2 de 4 subsecciones entre los primeros 5; las cuatro entre los primeros 10), las **fechas del expediente** (los Vistos con las fechas quedan en el lugar 6 o más) y la **norma aplicable** (pregunta de juicio jurídico que recupera otras secciones).

### El puntaje mínimo no separa lo que debía separar

Es el hallazgo principal. Las tres preguntas del dominio que el documento no responde —el precio del cobre, los sueldos, los paneles solares— obtuvieron puntajes de 0,633 a 0,688, por encima del piso de 0,60. Solo la pregunta ajena al dominio quedó bajo él.

Ningún umbral sirve para las dos cosas. Con 0,70 ninguna pregunta sin respuesta recibiría citas, pero tres respondibles quedarían sin su sustento. El piso calibrado en ADR-0011 con nueve preguntas filtra lo ajeno al dominio, y nada más.

Lo que evitó entregar citas falsas en esos tres casos fue el modelo: dijo que los pasajes no contenían la información, la respuesta no citó nada, y la verificación la descartó. La protección real contra preguntas sin respuesta es la generación, no el puntaje.

### Respuestas, revisadas a mano

Las revisó quien construyó el sistema, contra lo que cada pregunta mide.

| Pregunta | Qué pasó | Juicio |
|---|---|---|
| `aire-parametros` | Parámetros y límites correctos, citados | Correcta |
| `informes-frecuencia` | "Cada 6 meses", citado en las dos secciones que lo dicen | Correcta |
| `vida-util` | 11 años y cierre de tres | Correcta |
| `permisos-sectoriales` | Mixtos, artículos 138 y 160, sin los de contenido solo ambiental | Correcta |
| `sismo` | Las medidas del plan, citadas | Incompleta: omite el aviso a la SMA en 24 horas, que estaba en el segundo pasaje recuperado |
| `inversion` | USD 30 millones | Correcta |
| `compromisos-voluntarios` | Aire, aguas subterráneas y la charla de difusión | **Errónea**: omite aguas superficiales y ruido, e incluye la charla, que no es un compromiso voluntario |
| `estanque-agua-tratada` | Dice que el documento no explica la causa de la incongruencia y cita la exigencia de 4.000 l/día y 48 horas | Correcta: el mejor caso del conjunto |
| `estanque-tk65` | Modo degradado: el modelo dijo que los pasajes no respondían | Segura pero inútil: no confirmó la contradicción inexistente, y tampoco dio los 34 m³ que sí estaban en los pasajes |
| `fechas-expediente` | Modo degradado | Segura pero inútil: no dio las fechas |
| `limites-genericos` | Modo degradado: dijo que los pasajes no explican el porqué | Segura pero inútil: no confirmó el hecho, que sí estaba en los cuatro pasajes |
| `norma-aplicable` | Modo degradado | Segura |
| `plazos-contingencias` | Modo degradado: el modelo afirmó que los pasajes no mencionaban las 48 horas | Segura por diseño, pero el modelo se equivocó: el pasaje de la 12.3 lo dice. No se entregó porque no citaba |
| Las tres del dominio sin respuesta | Modo degradado: el modelo dijo que no había información | Seguras: ninguna afirmación falsa. Pero el texto genérico presenta como "más relacionados" pasajes que no tienen relación |
| `sin-respuesta-ajena` | "No se encontraron pasajes" | Correcta |

En resumen: **siete correctas, una incompleta, una errónea, y ocho seguras pero sin respuesta útil.** Ninguna afirmación falsa llegó a quien consulta, salvo la de los compromisos voluntarios, que pasó la verificación porque todas sus citas existían.

### Qué indica

1. **Las preguntas de revisión crítica terminan en modo degradado.** Ante un "¿por qué?" que el documento no responde, el modelo dice eso y nada más: no agrega lo que los pasajes sí dicen. Sin citas, la respuesta se descarta. La respuesta al estanque de agua tratada muestra que el modelo puede hacerlo bien cuando cita.
2. **El texto del modo degradado no distingue dos situaciones distintas**: que el modelo no haya respondido, y que haya respondido que los pasajes no contienen la respuesta.
3. **Las enumeraciones necesitan más pasajes**, y un encabezado de una línea ocupa el primer lugar sin aportar contenido.
4. **El puntaje mínimo solo filtra lo ajeno al dominio.**

Estas mejoras se miden contra este mismo resultado, para saber si mejoran o empeoran.
