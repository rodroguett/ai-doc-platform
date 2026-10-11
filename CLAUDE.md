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

- `v0.1.0` y `v0.2.0` completados. Sigue `v0.3.0`: ingesta end-to-end.
- El sistema es **un solo binario**, no cuatro servicios, y así seguirá
  hasta que se cumplan las condiciones de extracción de ADR-0005.
- El gateway implementa la interfaz generada desde el contrato OpenAPI.
  Los endpoints de documentos y trabajos operan sobre Postgres a través de
  `rag.Service`; `POST /v1/queries` responde con datos de ejemplo y
  `GET /v1/usage` devuelve 501. Los errores respetan RFC 9457.
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
- El contenido de un documento subido se usa para calcular su hash y luego
  se descarta: todavía no se guarda en ninguna parte (#24).
- Leer y consultar es público. Subir y eliminar documentos exige el token
  de escritura (`API_WRITE_TOKEN`; en Cloud Run, el secreto
  `api-write-token`). Sin token configurado, esas operaciones rechazan todo.
- No hay ingesta, ni búsqueda, ni llamadas a modelos.
- Desplegado en Cloud Run, público, con despliegue continuo desde `main`.

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
make test                # tests con detector de carrera; requiere Docker
make db-down             # elimina el Postgres local
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
todavía en `internal/gateway/service`. Los handlers traducen entre ambos. Es deliberado: permite cambiar el contrato HTTP sin
tocar la lógica, y al revés.

**Las consecuencias negativas se escriben.** En los ADRs y en las
descripciones de pull request, lo que quedó mal resuelto o pendiente se
declara. Un registro donde todas las decisiones salieron bien no es un
registro.

**El README debe describir el sistema que existe**, no el planificado. Si un
cambio hace que deje de ser cierto, se corrige en el mismo pull request.

## Presupuesto

El sistema usa APIs de pago con saldo prepagado y sin recarga automática. El
gasto previsto es inferior a un dólar mensual. Cualquier cambio que aumente
el consumo de tokens necesita justificarse; el caché de respuestas y el
conteo de costos son parte del diseño, no adornos.
