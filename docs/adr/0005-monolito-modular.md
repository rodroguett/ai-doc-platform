# ADR-0005: Construir como monolito modular antes de extraer servicios

- **Estado:** aceptado
- **Fecha:** 2026-09-30
- **Issue relacionado:** #22
- **Modifica:** [ADR-0001](0001-monorepo.md)

## Contexto

El [ADR-0001](0001-monorepo.md) describe el sistema como cuatro servicios
desplegables de forma independiente —gateway, llmgw, rag y orchestrator—
que se comunican por gRPC, y el README los dibuja así. Hoy existe uno solo:
el gateway, desplegado en Cloud Run, con los dos endpoints del camino
crítico respondiendo con datos de ejemplo detrás de interfaces.

Los próximos issues (#23, #24 y #25) construyen la persistencia, la ingesta
y la búsqueda. Antes de empezarlos hay que decidir dónde vive ese código y
cómo se comunica con el gateway, porque la respuesta cambia la forma de
cada uno.

Las fuerzas relevantes:

- Un solo autor. Cada servicio desplegado es un pipeline, una configuración
  y un conjunto de fallos más que mantener.
- Un presupuesto inferior a un dólar mensual. Cloud Run cobra por instancia
  activa; cuatro servicios son cuatro arranques en frío por consulta en el
  peor caso y cuatro topes de instancias que vigilar.
- Las fronteras entre los componentes todavía no se han probado con código
  real. Dónde termina la búsqueda y dónde empieza la generación, o quién
  cuenta los tokens de los embeddings, son preguntas que se responden
  construyendo, no dibujando.
- El proyecto quiere mostrar una arquitectura por servicios. Que esa
  arquitectura exista antes de ser necesaria no la hace más creíble.
- La regla de ADR-0001 según la cual un servicio no importa a otro "se
  verifica en CI con una comprobación de dependencias". Esa comprobación
  nunca se implementó: la regla se ha sostenido solo porque hasta ahora
  había un único servicio.

## Decisión

El sistema se construye como un solo binario compuesto por módulos con
fronteras explícitas y verificadas. Un módulo se extrae como servicio
independiente solo cuando se cumple alguna de las condiciones de la
sección Revisión.

Los módulos son los cuatro que ya nombra ADR-0001, ahora como paquetes
dentro del mismo proceso:

| Módulo | Responsabilidad | Puede importar |
|---|---|---|
| `internal/gateway` | HTTP, contrato OpenAPI, errores RFC 9457 | `orchestrator`, `rag` |
| `internal/orchestrator` | Encadena recuperación y generación para responder una consulta | `rag`, `llmgw` |
| `internal/rag` | Ingesta, fragmentación, embeddings, búsqueda en pgvector | `llmgw` |
| `internal/llmgw` | Proveedores de modelos, caché, conteo de costos, fallback | — |
| `internal/platform` | Configuración y utilidades compartidas | — |

Todos pueden importar `internal/platform`. Nadie importa `internal/gateway`:
es el borde del sistema, no una dependencia.

### Qué es una frontera

El **paquete raíz** de cada módulo es su contrato: los tipos que cruzan la
frontera, las interfaces que el módulo ofrece y su constructor. Es lo único
que otro módulo puede importar. Los subpaquetes (`internal/rag/store`,
`internal/rag/chunk`) son implementación privada.

El paquete raíz cumple el papel que ADR-0001 asignaba a `proto/`. Cuando un
módulo se extraiga, su paquete raíz se convierte en la definición gRPC y en
un cliente que implementa las mismas interfaces; los consumidores no
cambian.

Por eso el paquete raíz debe mantenerse delgado: tipos de datos,
interfaces y constructores, sin dependencias pesadas. Todo lo que no
sobreviviría a una serialización —punteros a estado compartido, funciones
como argumento, contextos con valores implícitos— no cruza la frontera.

### Dónde viven las interfaces

Quien consume define la interfaz que necesita, como ya hace el gateway con
`QueryService` y `DocumentService`. El módulo proveedor la satisface con los
tipos de su paquete raíz. El binario en `cmd/` es el único lugar que conoce
todas las implementaciones y las conecta.

El paquete `internal/gateway/service` es transitorio. Sus tipos de dominio
(`Query`, `Answer`, `Upload`, `Job`) se trasladan al paquete raíz de
`orchestrator` y de `rag` cuando esos módulos existan en #24 y #25, y sus
implementaciones de ejemplo desaparecen con ellos.

### Cómo se verifica

La regla se comprueba con `depguard` en golangci-lint, que ya corre en CI.
Por cada módulo, `.golangci.yml` declara qué importaciones están
prohibidas:

- los subpaquetes de otros módulos, siempre;
- el paquete raíz de los módulos que no figuran en su columna "Puede
  importar";
- `internal/gateway` desde cualquier módulo.

Dentro del gateway se agrega una regla más, que ya era convención: el
dominio (`internal/gateway/service`) no importa los tipos generados
(`internal/gateway/api`).

## Alternativas consideradas

### Extraer los servicios desde el inicio

Es la arquitectura que describen ADR-0001 y el README: cuatro servicios en
Cloud Run, gRPC entre ellos y NATS para la ingesta.

Se descartó porque paga todos los costos de la distribución —latencia de
red, fallos parciales, cuatro despliegues, infraestructura de mensajería—
antes de obtener ninguno de sus beneficios. Ningún componente necesita
todavía escalar ni desplegarse por separado. Y el costo más alto es otro:
si una frontera resulta estar mal trazada, corregirla dentro de un proceso
es un refactor; entre servicios, es cambiar un contrato gRPC y coordinar
dos despliegues.

### Monolito sin fronteras

Un solo paquete, o paquetes que se importan libremente entre sí.

Es lo más rápido a corto plazo. Se descartó porque deja la extracción como
una tarea de desenredar, no de mover, y porque el proyecto perdería lo que
quiere mostrar: que las fronteras están pensadas aunque todavía no sean de
red.

### Barreras físicas con el directorio `internal/` de Go

Poner la implementación de cada módulo bajo `internal/<módulo>/internal/`,
de modo que el compilador impida importarla desde fuera.

Es una barrera más fuerte que un linter. No se adoptó porque solo protege
los subpaquetes: no impide que `rag` importe el paquete raíz de
`orchestrator`, que es la otra mitad de la regla, y exige igualmente la
comprobación con `depguard`. Queda disponible como refuerzo si la
comprobación resulta insuficiente.

## Consecuencias

### Positivas

- Un solo despliegue, un solo arranque en frío y un solo tope de
  instancias. El sistema sigue escalando a cero dentro del presupuesto.
- Corregir una frontera mal trazada cuesta un refactor, no una migración
  de contrato entre servicios.
- La regla de dependencias entre módulos pasa a verificarse en CI, como
  ADR-0001 decía que ocurría.

### Negativas

- **Las llamadas en proceso ocultan los modos de fallo de la red.** No hay
  latencia, serialización, timeouts ni fallos parciales entre módulos. Una
  frontera que funciona bien dentro del proceso puede resultar demasiado
  conversacional o frágil al extraerse, y eso solo se descubrirá entonces.
- **Un solo dominio de fallo.** Un pánico o un consumo excesivo de memoria
  en la ingesta afecta a las consultas. Esto ya es concreto: el gateway lee
  archivos de hasta 20 MiB en memoria.
- **La ingesta asíncrona choca con Cloud Run.** Por omisión, Cloud Run solo
  asigna CPU mientras atiende una petición; una goroutine que procesa un
  documento después de responder 202 puede quedar congelada o terminar
  junto con la instancia. Resolverlo —CPU siempre asignada, Cloud Tasks o
  una cola en Postgres— queda para #24, y cualquiera de las opciones tiene
  costo o complejidad.
- **La independencia sigue siendo una convención verificada, no una
  barrera.** `depguard` compara prefijos de ruta: un módulo nuevo exige
  agregar sus reglas a `.golangci.yml`, y olvidarlo deja ese módulo sin
  verificar sin que nada falle.
- **Los contratos en Go no se versionan como los de proto.** Cambiar un
  tipo del paquete raíz rompe a sus consumidores en compilación, lo que
  dentro de un proceso es una ventaja, pero no ensaya la compatibilidad
  hacia atrás que exigirá un contrato gRPC.
- **ADR-0001 afirmaba una verificación que no existía.** La brecha estuvo
  abierta desde agosto sin que nada la detectara. Se corrige aquí, pero
  queda como antecedente de que una afirmación en un ADR no se verifica
  sola.
- **El README describe hoy una arquitectura que no existe.** Se corrige en
  #21.

### Neutras

- El binario sigue llamándose `gateway` aunque contendrá los cuatro
  módulos. Renombrarlo implica cambiar el servicio de Cloud Run y el
  workflow de despliegue; se posterga hasta que haya un motivo además del
  nombre.
- Los alcances de commit (`gateway`, `rag`, `llmgw`, `orchestrator`) de
  CONTRIBUTING.md siguen siendo válidos: nombran módulos en lugar de
  servicios.
- `proto/` no existirá hasta la primera extracción.

## Revisión

Un módulo se extrae como servicio cuando se cumple al menos una de estas
condiciones, y su contrato no ha cambiado en los últimos pull requests que
lo tocaron:

- **Necesita recursos distintos.** El candidato más probable es la
  ingesta: consume memoria y CPU durante minutos, un perfil incompatible
  con el de las consultas.
- **Necesita aislar fallos.** Si la latencia o los errores de un proveedor
  de modelos degradan endpoints que no lo usan.
- **Aparece un segundo consumidor** que no sea el gateway.
- **Aparece un segundo colaborador** con responsabilidad sobre un módulo,
  la misma condición que reabre ADR-0001.

La decisión completa se reconsidera si las reglas de `depguard` resultan
insuficientes para sostener las fronteras. En ese caso, lo primero a
evaluar es la barrera con `internal/` descrita en las alternativas.
