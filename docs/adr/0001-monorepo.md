# ADR-0001: Alojar todos los servicios en un monorepo

- **Estado:** aceptado
- **Fecha:** 2026-08-05
- **Issue relacionado:** #1

## Contexto

El sistema se compone de cuatro servicios desplegables de forma
independiente: gateway, llmgw, rag y orchestrator. Comparten contratos
gRPC, configuración, instrumentación y utilidades de infraestructura.

Las fuerzas relevantes:

- Un solo autor y un solo ciclo de release.
- Los contratos entre servicios cambian con frecuencia durante el diseño
  inicial, y cada cambio afecta a dos o más servicios a la vez.
- El sistema debe poder levantarse completo en local con un comando.
- El repositorio también cumple una función de comunicación: alguien
  externo debe poder entender el sistema completo sin recorrer varios
  orígenes.

## Decisión

Todos los servicios viven en un único repositorio, con un solo módulo Go
en la raíz. Cada servicio tiene su binario en `cmd/<servicio>` y su
implementación en `internal/<servicio>`. Lo compartido vive en
`internal/platform`.

La independencia entre servicios se mantiene por convención, no por
separación física: `internal/<servicio>` no importa a otro
`internal/<servicio>`. La única frontera permitida son los contratos de
`proto/` y `api/`. Esta regla se verifica en CI con una comprobación de
dependencias.

## Alternativas consideradas

### Un repositorio por servicio

Es lo habitual cuando hay equipos distintos por servicio, ciclos de
release desacoplados o permisos diferenciados. Ninguna de esas
condiciones aplica aquí.

Se descartó porque un cambio de contrato exigiría coordinar cuatro pull
requests, y porque la orquestación local (docker compose, Makefile) no
tendría un lugar natural donde vivir.

### Monorepo con múltiples módulos Go

Daría aislamiento real de dependencias entre servicios. Se descartó
porque a esta escala el costo de mantener varios `go.mod` y sus
`replace` supera el beneficio, y porque no hay conflictos de versiones
que resolver.

Es la alternativa a reconsiderar primero si el proyecto crece.

## Consecuencias

### Positivas

- Un cambio de contrato y sus consumidores entran en un solo commit
  atómico y revisable.
- El sistema completo se levanta con `docker compose up`.
- La documentación, los ADRs y los diagramas quedan junto al código que
  describen.

### Negativas

- El CI construye y prueba de más si no se filtra por rutas modificadas.
  Se mitiga con filtros por path en los workflows.
- Nada impide técnicamente que un servicio importe a otro. Depende de una
  verificación en CI, que es más frágil que una barrera física.
- El historial de commits mezcla los cuatro servicios, lo que hace menos
  directo seguir la evolución de uno solo. Se mitiga con el alcance en
  los mensajes de commit.

### Neutras

- El versionado es único para todo el sistema. Los servicios no tienen
  versiones independientes.

## Revisión

Reconsiderar si aparece un segundo colaborador con responsabilidad
exclusiva sobre un servicio, o si algún servicio necesita un ciclo de
release distinto del resto.
