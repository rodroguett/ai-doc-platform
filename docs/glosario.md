# Glosario

Términos del dominio regulatorio y del proyecto que no son de uso general.
Donde un término tiene un significado propio en este sistema, la
definición dice cuál es y enlaza a la decisión que lo fija.

- [Dominio regulatorio](#dominio-regulatorio)
- [Recuperación y respuestas](#recuperación-y-respuestas)
- [Arquitectura y operación](#arquitectura-y-operación)

## Dominio regulatorio

**Adenda.** Respuesta del titular a un ICSARA, en la que aclara, rectifica o
amplía lo que presentó. El proyecto Tres Valles tuvo dos: abril y noviembre
de 2025.

**Compromiso ambiental voluntario.** Medida que el titular asume sin que una
norma se la exija, y que pasa a ser obligatoria al quedar en la RCA. En la
RCA de prueba están en el considerando 8.

**Considerandos.** Parte central de una resolución, con los fundamentos de la
decisión numerados. En una RCA ahí están la descripción del proyecto, los
compromisos y los planes de seguimiento; por eso casi todas las citas dicen
"Considerando 10.1".

**DIA (Declaración de Impacto Ambiental).** Forma de ingresar un proyecto al
SEIA cuando el titular declara que no genera los efectos más graves que
define la ley. Es la vía del proyecto Tres Valles. Ver también *EIA*.

**Documento vinculante.** El que obliga al titular y fiscaliza la autoridad.
En el corpus es la RCA; los ICSARA son historial de la evaluación: una
solicitud que pudo acogerse o no. Ver *prioridad del documento vinculante*.

**EIA (Estudio de Impacto Ambiental).** Forma de ingresar al SEIA un
proyecto que sí genera los efectos más graves que define la ley. Es más
extenso que una DIA.

**ICE (Informe Consolidado de Evaluación).** Informe con que el SEA cierra la
evaluación y recomienda aprobar o rechazar el proyecto.

**ICSARA (Informe Consolidado de Solicitud de Aclaraciones, Rectificaciones
y/o Ampliaciones).** Las preguntas formales que el SEA le hace al titular
durante la evaluación. Se divide en capítulos con numeración romana. El
corpus tiene los dos del proyecto Tres Valles.

**PAC (Participación Ciudadana).** Etapa de la evaluación en que la
comunidad puede hacer observaciones al proyecto.

**PAS (Permiso Ambiental Sectorial).** Permiso que otorga otro organismo del
Estado, pero cuyos aspectos ambientales se evalúan dentro del SEIA. Se
identifican por artículo del reglamento: "PAS 138".

**Plan de seguimiento.** Conjunto de monitoreos que el titular debe hacer y
reportar para mostrar que cumple, con su frecuencia y sus límites.

**RCA (Resolución de Calificación Ambiental).** Resolución que aprueba o
rechaza un proyecto y fija las condiciones que el titular debe cumplir. Es
el documento central del corpus.

**Resuelvo.** Parte final de una resolución, con lo que la autoridad decide.
En las citas aparece como "Resuelvo 5".

**SEA (Servicio de Evaluación Ambiental).** Organismo que administra el SEIA
y publica sus expedientes.

**SEIA (Sistema de Evaluación de Impacto Ambiental).** Procedimiento por el
que un proyecto se evalúa antes de ejecutarse.

**SMA (Superintendencia del Medio Ambiente).** Organismo que fiscaliza que
el titular cumpla su RCA.

**Titular.** Quien presenta el proyecto y queda obligado por la RCA. En el
corpus, Minera Tres Valles SpA.

**Vistos.** Parte inicial de una resolución, que enumera los antecedentes y
las normas en que se basa. En las citas aparece como "Vistos 4".

## Recuperación y respuestas

**Búsqueda híbrida.** Combinar la búsqueda por similitud con una por texto
completo. Se midió y se descartó: encuentra pasajes que la similitud pierde,
pero empeora las enumeraciones
([ADR-0016](adr/0016-no-combinar-vectores-y-texto.md)).

**Búsqueda por similitud.** Encontrar los fragmentos cuyo embedding se
parece más al de la pregunta. Aquí es exacta, sin índice aproximado, y la
medida es la similitud coseno, entre 0 y 1
([ADR-0011](adr/0011-recuperacion-y-modo-degradado.md)).

**Caché de respuestas.** Las respuestas a una pregunta repetida en los
últimos 10 minutos salen de memoria, sin gastar cuota ni tokens.

**Capa gratuita.** Plan sin costo de un proveedor, con cuotas diarias. El
proyecto usa la de Gemini y la de Neon, sin medio de pago: al agotarse una
cuota el servicio falla en lugar de cobrar.

**Cifra sin respaldo.** Una cifra de la respuesta que no aparece en los
pasajes que cita su oración. La respuesta se entrega con un aviso que la
nombra ([ADR-0014](adr/0014-verificar-cifras-de-las-citas.md)).

**Cita.** El pasaje que sustenta una afirmación, con su documento, sección,
páginas y extracto. En el texto se marca con `[n]`.

**Colección.** Agrupación de documentos dentro del corpus. Una consulta
busca en una colección; la de producción es `rca`.

**Conjunto de evaluación.** Las preguntas con que se mide el sistema, cada
una con las secciones que la responden (`eval/rca-tres-valles.json`). Hoy
son 26 ([resultados](eval/README.md)).

**Contexto de un fragmento.** Lo que el fragmento no dice de sí mismo y lo
acompaña al buscar y al redactar: la introducción del ítem al que pertenece
o el título de su capítulo.

**Corpus.** El conjunto de documentos sobre el que el sistema responde. En
la demo, la RCA y los dos ICSARA del proyecto Tres Valles.

**Costo estimado.** Lo que habría costado una consulta con la tarifa de
lista del proveedor en su capa de pago. No es lo que se paga, que es cero
([ADR-0013](adr/0013-costo-estimado-por-consulta.md)).

**Embedding.** Vector numérico que representa el significado de un texto.
Cada fragmento tiene uno, de 768 dimensiones
([ADR-0008](adr/0008-embeddings-gemini.md)).

**Fragmento (chunk).** Unidad en que se divide un documento para buscarlo:
un considerando, una subsección o una solicitud de un ICSARA, de a lo más
3.500 caracteres.

**Frase de ausencia.** Una oración que solo dice qué no contienen los
pasajes ("los pasajes no mencionan…"). Sus marcas de cita no cuentan,
porque no señalan nada.

**Ingesta.** Extraer el texto de un PDF, fragmentarlo y generar el embedding
de cada fragmento. Corre en segundo plano
([ADR-0010](adr/0010-trabajos-con-cloud-tasks.md)).

**Línea base.** El resultado de la evaluación contra el que se compara un
cambio. La primera es del 2026-10-14.

**Marca de cita.** El `[n]` que cierra una afirmación e indica qué pasaje la
sustenta.

**Modelo de respaldo.** El modelo que redacta cuando el principal no
responde: `gemini-3.1-flash-lite`
([ADR-0012](adr/0012-generacion-gemini-flash-lite.md)).

**Modo degradado.** Respuesta sin texto redactado: se entregan los pasajes,
y la traza lo declara. Ocurre cuando ningún modelo responde, o cuando el que
responde no cita nada.

**Mutación (prueba de mutación).** Romper a propósito una parte del código
para comprobar que algún test falla. Se usa para verificar que las pruebas
nuevas detectan lo que dicen detectar.

**Pasaje.** Un fragmento recuperado para una pregunta, con su puntaje.

**Premisa falsa.** Una pregunta que da por cierto algo que el documento no
dice, como una cifra cambiada. La respuesta no debe repetirla como cierta.

**Prioridad del documento vinculante.** Al ordenar los pasajes, los de la
RCA suman 0,03 a su similitud. Solo cambia el orden, no el puntaje
([ADR-0015](adr/0015-priorizar-el-documento-vinculante.md)).

**Puntaje mínimo (piso para citar).** La similitud bajo la cual un pasaje
no se cita: 0,60. Filtra lo ajeno al dominio, no las preguntas del dominio
que el corpus no responde.

**RAG (generación aumentada por recuperación).** Responder con un modelo a
partir de pasajes recuperados de un corpus, en vez de lo que el modelo sabe
por sí mismo.

**Temperatura.** Cuánto varía un modelo al redactar. Aquí es 0, para que la
misma pregunta dé, en lo posible, la misma respuesta.

**Token.** Unidad en que un modelo cuenta el texto; en español, unos cuatro
caracteres. Las cuotas y las tarifas se miden en tokens.

**Tope diario.** Cuántas consultas nuevas se atienden por día: 500. Se lee
del registro de uso, así que sobrevive a los reinicios.

**Traza.** Lo que acompaña a cada respuesta para saber cómo se produjo:
modelo, tokens, costo estimado, latencia, si vino de la caché y las cifras
sin respaldo.

**Verificación de citas.** Lo que se comprueba antes de entregar una
respuesta: la *forma* (cada marca apunta a un pasaje entregado) y las
*cifras* (están en lo que se cita). El *texto*, que el pasaje diga lo que
la oración afirma, no se verifica
([ADR-0014](adr/0014-verificar-cifras-de-las-citas.md)).

## Arquitectura y operación

**ADR (Architecture Decision Record).** Registro de una decisión de diseño,
con sus alternativas y sus consecuencias, incluidas las negativas. Están en
[`docs/adr`](adr/).

**Cloud Run.** Servicio de Google Cloud que ejecuta el binario y escala a
cero cuando no hay tráfico. Corre con una sola instancia como máximo.

**Cloud Tasks.** Cola de Google Cloud que despacha las pasadas de la ingesta
([ADR-0010](adr/0010-trabajos-con-cloud-tasks.md)).

**Content-Security-Policy.** Encabezado que limita lo que una página puede
ejecutar y a dónde puede conectarse. La de la demo solo admite su propio
script, por su hash.

**Contrato.** La especificación de la API en `api/openapi.yaml`, de la que se
genera el código del servidor. Ver *spec-first*.

**depguard.** Linter que verifica las fronteras entre módulos: cada uno
importa solo el paquete raíz de los otros, y nadie importa el gateway
([ADR-0005](adr/0005-monolito-modular.md)).

**Despliegue continuo.** Cada merge a `main` se despliega a producción. Un
release es otra cosa: agrupa varios cambios
([ADR-0003](adr/0003-github-flow.md)).

**gateway, orchestrator, rag, llmgw.** Los cuatro módulos del sistema: la
API HTTP; la consulta y la verificación de la respuesta; los documentos,
los fragmentos y la búsqueda; y el acceso a los modelos.

**GitHub Flow.** Flujo de trabajo con ramas de vida corta que entran a
`main` por pull request ([ADR-0003](adr/0003-github-flow.md)).

**Migración.** Cambio versionado del esquema de la base de datos. Se aplican
al arrancar, y cada módulo anota las suyas en su propia tabla
([ADR-0006](adr/0006-postgres-pgvector.md)).

**Monolito modular.** Un solo binario dividido en módulos con fronteras
verificadas, que se extraen como servicios solo si una condición concreta lo
justifica ([ADR-0005](adr/0005-monolito-modular.md)).

**Neon.** Proveedor de Postgres administrado, en su capa gratuita.

**OpenAPI.** Formato estándar para describir una API HTTP; es el formato del
contrato.

**PDFium.** Motor de lectura de PDF de Chromium, que aquí corre en
WebAssembly para extraer el texto ([ADR-0009](adr/0009-extraccion-pdfium.md)).

**pgvector.** Extensión de Postgres que guarda vectores y busca por
similitud ([ADR-0006](adr/0006-postgres-pgvector.md)).

**RFC 9457 (Problem Details).** Formato estándar para los errores de una API
HTTP: tipo, título, estado y detalle.

**Spec-first.** Escribir primero el contrato y generar el código desde él,
no al revés ([ADR-0004](adr/0004-spec-first.md)).

**Token de escritura.** Credencial que exigen las operaciones que escriben,
como subir o borrar documentos. Leer y consultar es público
([ADR-0007](adr/0007-token-de-escritura.md)).
