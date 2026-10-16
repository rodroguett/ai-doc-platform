# Contexto del proyecto

Sistema de consulta sobre documentos regulatorios que responde en lenguaje
natural citando la fuente exacta, con control explícito del costo por
consulta y degradación controlada cuando el proveedor de generación falla.

El dominio que guía el diseño es el cumplimiento ambiental en la minería
chilena: una Resolución de Calificación Ambiental contiene cientos de
compromisos vinculantes repartidos entre el documento original, sus adendas
y los informes de seguimiento.

**La restricción que ordena la arquitectura**: en un sector fiscalizado, la
trazabilidad de la respuesta importa más que su fluidez. Una respuesta sin
fuente verificable no sirve. Por eso cada respuesta lleva citas al documento
y sección de origen, y una traza de cómo se produjo.

Es un proyecto de portafolio con un solo autor. El repositorio cumple una
función de comunicación además de alojar código: el historial, los pull
requests y los ADRs son parte de lo que el proyecto muestra.

## Estado actual

Lee esta sección con escepticismo: puede estar desactualizada. Verifica
contra el código y contra `gh issue list`.

- `v0.1.0` a `v0.4.0` completados. `v0.4.0` es la consulta con citas
  reales. No hay un milestone siguiente definido.
- El sistema es **un solo binario**, no cuatro servicios, y así seguirá
  hasta que se cumplan las condiciones de extracción de ADR-0005.
- El gateway implementa la interfaz generada desde el contrato OpenAPI.
  Todas las operaciones están implementadas salvo `GET /v1/usage`, que
  devuelve 501. Los errores respetan RFC 9457. Ya no hay datos de ejemplo.
- `/health` indica que el proceso está vivo y `/ready` verifica las
  dependencias (hoy, la base de datos). No usar rutas terminadas en `z`:
  Cloud Run las intercepta.
- El contrato se sirve en `/openapi.yaml` y Swagger UI en `/docs`, ambos
  fuera de `/v1`. Swagger UI se carga desde un CDN con versión y hash de
  integridad fijos en `internal/gateway/swagger.html`.
- Hay base de datos: Postgres con pgvector en Neon. El binario conecta y
  migra al arrancar si `DATABASE_URL` está definida (en Cloud Run viene del
  secreto `database-url`); sin ella, los endpoints de documentos responden
  503.
- Al subir un PDF, `rag.Service.Submit` extrae su texto, lo fragmenta,
  guarda documento, trabajo y fragmentos en una transacción y encola el
  trabajo, todo dentro de la misma petición. El PDF original no se
  conserva: solo su hash y sus fragmentos.
- `rag.Service.RunJob` ejecuta una pasada de un minuto: genera embeddings
  de los fragmentos pendientes y los guarda de a ocho. Lo invoca
  `POST /v1/jobs/{id}/run`, protegida con el token de escritura. En Cloud
  Run esa petición la hace Cloud Tasks (cola `ingest`, variables
  `CLOUD_TASKS_QUEUE` y `SERVICE_URL`); sin cola, una goroutine
  (`internal/rag/dispatch`). ADR-0010.
- El gateway acepta una subida a la vez (503 a la segunda): la memoria de
  la instancia alcanza para el motor de extracción y un archivo, no para
  varios.
- Leer y consultar es público. Subir y eliminar documentos exige el token
  de escritura (`API_WRITE_TOKEN`; en Cloud Run, el secreto
  `api-write-token`). Sin token configurado, esas operaciones rechazan todo.
- La extracción (`internal/rag/pdftext`) y la fragmentación
  (`internal/rag/chunk`) se prueban con una RCA real en
  `internal/rag/testdata`. La introducción de un ítem con subsecciones, si
  tiene hasta 400 caracteres, no es fragmento propio: va en `Chunk.Context`
  de cada subsección, se antepone al texto del embedding y se le muestra al
  modelo. Cambiar el fragmentador no altera lo ya ingerido: hay que borrar
  y volver a subir el documento.
- `internal/llmgw` genera embeddings con Gemini (`internal/llmgw/gemini`),
  por HTTP y sin SDK. Reintenta fallas transitorias, regula el ritmo a
  24.000 tokens por minuto y cuenta su propio consumo. `rag` lo consume a
  través de su propia interfaz `rag.Embedder`. También redacta
  (`gemini.Generator`, que implementa `llmgw.Generator`).
- `internal/orchestrator` responde las consultas: pide los pasajes a
  `rag.Service.Search`, descarta los que puntúan bajo 0,60, le pide la
  redacción a `llmgw.Generator` y verifica las citas (`verifyCitations`):
  toda marca `[n]` debe apuntar a un pasaje entregado, y debe haber al
  menos una. Antes descarta las marcas de las frases que solo declaran una
  ausencia ("los pasajes no contienen…"), reconocidas con una expresión
  regular (`absenceClaim`, `affirms`). El prompt pide respuestas parciales:
  decir qué no responden los pasajes y resumir lo que sí dicen. Devuelve como citas solo los pasajes citados, con las marcas
  renumeradas. Si el modelo falla o su texto no pasa la verificación,
  responde en modo degradado (los pasajes, `degraded: true`), con un texto
  distinto si el modelo concluyó que los pasajes no responden; nunca es un
  error. Caché de 10 minutos y tope de 500 consultas nuevas por día, en
  memoria. ADR-0011.
- Cada consulta atendida deja una fila en `query_usage` (ADR-0013): modelo,
  tokens, caché, desenlace y costo estimado a tarifa de lista, sin la
  pregunta. Lo escribe `internal/orchestrator/store` a través de
  `orchestrator.UsageRecorder`; si falla, la consulta se responde igual.
  Las tarifas están en `internal/llmgw/gemini/prices.go`; un modelo sin
  tarifa deja el costo nulo, nunca cero. La traza lo expone como
  `estimated_cost_usd`.
- Cada módulo con esquema propio anota sus migraciones en su tabla de
  versiones: `rag` en `goose_db_version`, `orchestrator` en
  `orchestrator_goose_version` (`postgres.MigrateTable`).
- La verificación de citas es de forma: no comprueba que el pasaje diga lo
  que la respuesta afirma.
- La búsqueda es exacta, sin índice sobre los vectores, a propósito.
- Antes de convertir una pregunta en vector, `rag` agrega a cada sigla del
  SEIA su nombre completo (`internal/rag/acronyms.go`). Solo afecta la
  búsqueda: la pregunta que ve el modelo no cambia.
- En producción está ingerida la RCA "Continuidad Operacional Minera Tres
  Valles" (colección `rca`). Con el fragmentador actual son 159 fragmentos.
- La calidad se mide con `make eval` contra `eval/rca-tres-valles.json`
  (19 preguntas). Resultado y lectura en `docs/eval/README.md`: el puntaje
  mínimo no separa las preguntas del dominio sin respuesta; lo que las
  detiene es la generación. Cualquier cambio a la búsqueda, al prompt o al
  umbral se mide contra el resultado vigente, y el informe se corrige a
  mano: la verificación automática no juzga si la respuesta es correcta.
  Un caso puede declarar `reject`: secciones que la respuesta no debe citar.
- **La evaluación tiene su propia clave de Gemini**, en otro proyecto sin
  facturación, en `~/.config/ai-doc-platform/eval-gemini-key`; `make eval`
  no corre sin ella. Nunca medir con la clave de producción: la cuota es de
  1.000 solicitudes de embeddings por día y por proyecto, una ingesta gasta
  una por fragmento, y agotarla deja la demo sin búsqueda. Re-ingerir en
  producción también gasta de esa cuota. El aviso de reintento del
  proveedor no es confiable para saber cuándo se repone.
- Desplegado en Cloud Run, público, con despliegue continuo desde `main`,
  con una sola instancia como máximo, 512 MiB de memoria y 120 s de plazo
  por petición. Con 256 MiB el proceso muere al extraer el primer PDF.
- Límite de 60 peticiones por minuto por origen (ráfaga de 30), en memoria,
  activo por omisión; `/health` está exento. El origen es el último valor
  de `X-Forwarded-For`, que agrega Google y el cliente no puede falsear.

## Cómo está organizado

```
cmd/gateway/             el único binario; conecta las implementaciones
internal/<módulo>/       gateway, orchestrator, rag, llmgw
internal/platform/       código compartido entre módulos
internal/gateway/api/    GENERADO desde el contrato, no editar a mano
api/openapi.yaml         el contrato: fuente de verdad de la API
api/codegen.yaml         configuración de la generación
api/embed.go             embebe el contrato para servirlo desde el binario
docs/adr/                decisiones de arquitectura
scripts/bootstrap.sh     instala las herramientas en sus versiones exactas
```

Todos los módulos viven en un monorepo con un único módulo Go. Un módulo
solo puede importar el **paquete raíz** de los módulos de los que depende,
nunca sus subpaquetes, y nadie importa el gateway. La regla se verifica con
`depguard` en `make lint`; al crear un módulo nuevo hay que agregar sus
reglas a `.golangci.yml` (ADR-0005).

## Decisiones vigentes

Los ADRs en `docs/adr/` son la referencia. En resumen:

- **Monorepo** con un solo módulo Go (ADR-0001).
- **Versiones de herramientas fijadas** de forma exacta en `.tool-versions`,
  replicadas en el workflow de CI y el Dockerfile (ADR-0002). La versión de
  Go utilizable no es la última publicada, sino la más reciente que todas
  las herramientas soportan: golangci-lint 2.12.2 aborta con sintaxis de Go
  1.27, por lo que el proyecto permanece en la serie 1.25.
- **GitHub Flow** con despliegue continuo (ADR-0003). Un merge a `main`
  despliega; un release es otra cosa y agrupa varios cambios.
- **Spec-first** (ADR-0004): el contrato genera el código, no al revés.
- **Monolito modular** (ADR-0005): un binario con fronteras verificadas; un
  módulo se extrae como servicio solo cuando lo justifica una condición
  concreta.
- **Postgres con pgvector** como único almacén, migraciones SQL embebidas
  que se aplican al arrancar (ADR-0006). Toda migración debe ser compatible
  con el código de la revisión anterior: nada ejecuta el `Down`.
- **Embeddings con `gemini-embedding-001`, 768 dimensiones, en la capa
  gratuita de Gemini** (ADR-0008). La clave vive en un proyecto de Google
  sin facturación: es lo que impide un cobro. No asociarle facturación ni
  mover la clave al proyecto del servicio. Ningún fragmento puede superar
  los 2.048 tokens.
- **Extracción de PDF con PDFium en WebAssembly** (ADR-0009), sin CGO. El
  motor reserva unos 310 MiB, medidos; por eso el servicio corre con 512. `go-pdfium` está fijado en la
  1.19.8, la última que admite Go 1.25.
- **Trabajos en pasadas despachadas por Cloud Tasks** (ADR-0010). El
  avance vive en la base de datos: una pasada pregunta qué fragmentos no
  tienen vector y sigue desde ahí. No usar goroutines para trabajo en
  segundo plano en Cloud Run: la CPU se retira al responder. La cola
  `ingest` se creó a mano y no está en el repositorio.
- **Generación con `gemini-3.5-flash-lite`**, respaldo en
  `gemini-3.1-flash-lite`, y si ninguno responde, modo degradado
  (ADR-0012). Misma clave y misma capa gratuita que los embeddings. Los
  modelos Flash mayores devuelven 503 en la capa gratuita; no cambiarlos sin
  medir. Cada modelo se intenta una vez por consulta, sin reintentos.
- **Costo estimado, no real** (ADR-0013). Todo corre en capas gratuitas;
  el costo que se registra es lo que habría costado a tarifa de lista, y
  se rotula como estimado en todas partes. Cambiar de modelo exige agregar
  su tarifa.
- **Token único de escritura** (ADR-0007). Las operaciones protegidas se
  declaran en el contrato con `security` y se repiten en
  `protectedOperations` (`internal/gateway/auth.go`); un test verifica que
  coincidan. Una operación nueva que escriba debe agregarse en ambos.

## Convenciones

- Ramas: `<tipo>/<numero-issue>-<slug>`, de vida corta.
- Commits: Conventional Commits, asunto en imperativo, uno por unidad de
  cambio. El cuerpo explica el porqué; el diff ya muestra el qué.
- Todo cambio entra por pull request con squash. `main` está protegida.
- El detalle completo está en `CONTRIBUTING.md`.

## Comandos

```bash
./scripts/bootstrap.sh   # instala herramientas en las versiones declaradas
make run                 # levanta el gateway en :8080, sin base de datos
make run-db              # lo mismo, con un Postgres local en Docker
make test                # tests con detector de carrera; requiere Docker.
                         # Tarda uno o dos minutos por los tests de PDF
make db-down             # elimina el Postgres local
make eval                # mide búsqueda y respuestas; consume cuota de Gemini

# Prueba contra la API real de Gemini; consume cuota, no corre por omisión:
GEMINI_LIVE_TEST=1 GEMINI_API_KEY=... go test -run TestLive ./internal/llmgw/gemini/
make lint                # golangci-lint
make generate            # regenera el código desde el contrato
```

Tras editar `api/openapi.yaml` hay que ejecutar `make generate`. CI falla si
el código generado no corresponde al contrato.

## Al trabajar en este repositorio

**No edites `internal/gateway/api/openapi_gen.go`.** Cualquier cambio se
pierde en la siguiente generación. Para cambiar la API se edita el contrato
y se regenera.

**No subas la versión de Go por una dependencia.** `GOTOOLCHAIN=local` está
fijado en el Makefile y en CI para que `go get` falle en lugar de descargar
otro toolchain. Si una dependencia exige un Go más nuevo que el de
`.tool-versions`, se usa una versión anterior de la dependencia. Fuera del
Makefile, exporta `GOTOOLCHAIN=local` antes de usar `go get`.

**Verifica antes de afirmar.** Este proyecto ha acumulado fricción por dar
por hecho el estado de archivos y versiones. Antes de proponer un cambio,
lee el archivo; antes de dar por instalada una herramienta, comprueba su
versión.

**Los tipos del dominio están separados de los generados.** Los de
documentos y trabajos viven en el paquete raíz de `rag`; los de consultas,
en el de `orchestrator`. Los handlers traducen entre ambos. Es deliberado:
permite cambiar el contrato HTTP sin tocar la lógica, y al revés.

**Las consecuencias negativas se escriben.** En los ADRs y en las
descripciones de pull request, lo que quedó mal resuelto o pendiente se
declara. Un registro donde todas las decisiones salieron bien no es un
registro.

**El README debe describir el sistema que existe**, no el planificado. Si un
cambio hace que deje de ser cierto, se corrige en el mismo pull request.

## Presupuesto

El demo no debe generar ningún cobro; el tope acordado es un dólar. Cloud
Run es el único componente con facturación activa: lo acotan el máximo de
una instancia y una alerta de presupuesto de un dólar en la cuenta de
facturación, que avisa pero no detiene el gasto. No subir el máximo de
instancias sin revisarlo. La base
de datos (Neon) y los embeddings (Gemini) usan capas gratuitas sin medio de
pago, de modo que al agotarse una cuota el servicio falla en lugar de
cobrar. Cualquier cambio que introduzca un medio de pago, o que aumente el
consumo de una cuota, necesita justificarse; el caché de respuestas y el
conteo de costos son parte del diseño, no adornos.
