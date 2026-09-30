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

- `v0.1.0` y `v0.2.0` (parcial) completados.
- El sistema es **un solo binario**, no cuatro servicios. La extracción a
  servicios independientes está planificada, no hecha.
- El gateway implementa la interfaz generada desde el contrato OpenAPI. Los
  handlers devuelven "no implementado"; los errores sí respetan RFC 9457.
- No hay base de datos, ni ingesta, ni búsqueda, ni llamadas a modelos.
- Desplegado en Cloud Run, público, con despliegue continuo desde `main`.

## Cómo está organizado

```
cmd/<servicio>/          binario de cada servicio
internal/<servicio>/     implementación
internal/platform/       código compartido entre servicios
internal/gateway/api/    GENERADO desde el contrato, no editar a mano
api/openapi.yaml         el contrato: fuente de verdad de la API
api/codegen.yaml         configuración de la generación
docs/adr/                decisiones de arquitectura
scripts/bootstrap.sh     instala las herramientas en sus versiones exactas
```

Todos los servicios viven en un monorepo con un único módulo Go. La
independencia entre ellos se mantiene por convención: `internal/<servicio>`
no importa a otro `internal/<servicio>`.

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

## Convenciones

- Ramas: `<tipo>/<numero-issue>-<slug>`, de vida corta.
- Commits: Conventional Commits, asunto en imperativo, uno por unidad de
  cambio. El cuerpo explica el porqué; el diff ya muestra el qué.
- Todo cambio entra por pull request con squash. `main` está protegida.
- El detalle completo está en `CONTRIBUTING.md`.

## Comandos

```bash
./scripts/bootstrap.sh   # instala herramientas en las versiones declaradas
make run                 # levanta el gateway en :8080
make test                # tests con detector de condiciones de carrera
make lint                # golangci-lint
make generate            # regenera el código desde el contrato
```

Tras editar `api/openapi.yaml` hay que ejecutar `make generate`. CI falla si
el código generado no corresponde al contrato.

## Al trabajar en este repositorio

**No edites `internal/gateway/api/openapi_gen.go`.** Cualquier cambio se
pierde en la siguiente generación. Para cambiar la API se edita el contrato
y se regenera.

**Verifica antes de afirmar.** Este proyecto ha acumulado fricción por dar
por hecho el estado de archivos y versiones. Antes de proponer un cambio,
lee el archivo; antes de dar por instalada una herramienta, comprueba su
versión.

**Los tipos del dominio están separados de los generados.** Los handlers
traducen entre ambos. Es deliberado: permite cambiar el contrato HTTP sin
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
