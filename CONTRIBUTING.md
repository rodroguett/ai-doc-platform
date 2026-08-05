# Flujo de trabajo

Este documento describe cómo se trabaja en este repositorio. Aunque el
proyecto tiene un solo autor, el flujo es el mismo que se usaría en equipo:
el objetivo es que el historial sea legible y las decisiones queden trazadas.

## Ramas

`main` está protegida y siempre debe ser desplegable. Todo cambio entra por
pull request.

Nomenclatura de ramas:

```
<tipo>/<numero-issue>-<slug>
```

Ejemplos:

```
feat/12-query-endpoint
fix/34-pgx-pool-leak
docs/8-adr-monorepo
refactor/21-provider-interface
chore/40-otel-exporter
```

Una rama debería vivir menos de tres días. Si vive más, el issue estaba mal
dimensionado y conviene dividirlo.

## Commits

Se usa [Conventional Commits](https://www.conventionalcommits.org/).

```
<tipo>(<alcance>): <descripción en imperativo>

[cuerpo opcional explicando el porqué, no el qué]

Refs #12
```

Tipos aceptados: `feat`, `fix`, `refactor`, `perf`, `test`, `docs`,
`build`, `ci`, `chore`.

Alcances habituales: `gateway`, `llmgw`, `rag`, `orchestrator`, `platform`,
`api`, `deploy`.

Ejemplos:

```
feat(rag): add hnsw index on chunk embeddings
fix(llmgw): release provider slot when context is cancelled
refactor(llmgw): extract provider interface for multi-vendor support
docs(adr): record decision to use a monorepo
```

Reglas:

- El asunto en imperativo, sin punto final, máximo 72 caracteres.
- Un commit hace una sola cosa. Si la descripción necesita un "y", son dos
  commits.
- El cuerpo explica el motivo del cambio. El diff ya muestra el qué.
- `Closes #12` solo en el commit que efectivamente cierra el issue.

## Pull requests

Todo PR debe:

1. Referenciar un issue.
2. Pasar el CI (lint, tests, build, validación del contrato OpenAPI).
3. Tener descripción completada según la plantilla.
4. Entrar a `main` con **squash merge**, usando un asunto que también siga
   Conventional Commits.

El squash mantiene `main` legible: un commit por unidad de trabajo. El
detalle de la iteración queda visible dentro del PR.

Autorevisión: antes de mergear, se recorre el diff propio comentando las
decisiones no obvias. Los comentarios quedan como documentación del porqué.

## Releases

Un merge a `main` despliega automáticamente. Eso **no** es un release.

Un release agrupa varios cambios en un hito describible en una frase y se
marca con un tag semver:

```
git tag -a v0.3.0 -m "ingesta asíncrona de documentos end-to-end"
git push origin v0.3.0
```

El CHANGELOG se genera desde los commits convencionales. Mientras el
proyecto esté en `0.x`, la API puede cambiar entre versiones menores.

## Decisiones de arquitectura

Toda decisión con consecuencias estructurales se registra como ADR en
`docs/adr/`, numerada correlativamente, usando la plantilla `0000-template.md`.

Un ADR no se edita para cambiar de opinión: se crea uno nuevo que
supersede al anterior, y el original se marca como reemplazado. El valor
está en poder leer la evolución del razonamiento.

## Definición de terminado

Un issue se considera cerrado cuando:

- [ ] El código está en `main` y desplegado.
- [ ] Hay tests que cubren el comportamiento nuevo.
- [ ] El contrato OpenAPI refleja los cambios, si aplica.
- [ ] La documentación afectada está actualizada.
- [ ] Si hubo una decisión de diseño relevante, existe su ADR.
