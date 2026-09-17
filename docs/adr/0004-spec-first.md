# ADR-0004: Derivar el servidor del contrato OpenAPI

- **Estado:** aceptado
- **Fecha:** 2026-09-16
- **Issue relacionado:** #18

## Contexto

El gateway expone la superficie pública del sistema. Su contrato es lo que
consumirán los clientes y, más adelante, los demás servicios; su estabilidad
y su exactitud importan más que las de cualquier otra parte del código.

Un contrato solo sirve si describe lo que el servicio efectivamente hace. La
forma habitual en que deja de servir no es que alguien lo escriba mal, sino
que el código evoluciona y la especificación se queda atrás: un campo que
cambia de nombre, un código de respuesta nuevo, un parámetro que dejó de
usarse. Nada en el proceso obliga a actualizarla, así que no se actualiza, y
llegado cierto punto nadie confía en ella.

La pregunta es dónde ubicar la fuente de verdad: en el código, con la
especificación derivada de él, o en la especificación, con el código
derivado de ella.

## Decisión

El archivo `api/openapi.yaml` es la fuente de verdad. Los tipos de datos y
la interfaz que el servidor debe implementar se generan a partir de él con
oapi-codegen, y el gateway implementa esa interfaz.

La generación produce una interfaz estricta: cada operación recibe un objeto
de petición tipado y devuelve un tipo de respuesta que solo admite lo que el
contrato declara. Un handler no puede responder algo que la especificación
no contemple.

El paquete generado se acompaña de una aserción en tiempo de compilación:

```go
var _ api.StrictServerInterface = (*API)(nil)
```

Con ella, un endpoint declarado en el contrato y ausente en el código impide
construir el binario. El caso inverso —una especificación editada sin
regenerar— lo detecta CI, que regenera y compara contra lo commiteado.

Entre ambos mecanismos, el contrato y el código no pueden divergir sin que
algo falle de forma visible.

## Alternativas consideradas

### Code-first con anotaciones

Escribir los handlers y generar la especificación a partir de comentarios
estructurados sobre ellos, con swaggo o similar.

Es más rápido al principio y no introduce un paso de generación en el ciclo
de desarrollo. Se descartó porque desplaza el problema en lugar de
resolverlo: las anotaciones son comentarios, y nada obliga a mantenerlas al
día cuando el handler cambia. La especificación resultante es tan confiable
como la disciplina de quien edita el código, que es exactamente la situación
que se busca evitar.

### Mantener ambos a mano

Escribir la especificación y el código de forma independiente, verificando
la correspondencia con pruebas de contrato.

Es viable y en algunos equipos funciona bien. Se descartó porque requiere
construir y mantener esa capa de verificación, trabajo que aquí lo hace el
compilador sin costo adicional.

### Generar durante el build en lugar de commitear

Se optó por commitear el archivo generado. Commitearlo hace que el
repositorio compile sin requerir la herramienta instalada, y hace visibles
en el diff las consecuencias de cambiar el contrato: quien revisa un cambio
de especificación ve qué tipos y firmas se alteran.

El costo es que los diffs de un cambio de contrato incluyen un archivo
generado extenso, lo que dificulta revisar el cambio real entre el ruido.

## Consecuencias

### Positivas

- La divergencia entre contrato y código pasa de ser un descuido silencioso
  a un fallo de compilación o de CI.
- La documentación de la API es correcta por construcción, no por
  mantenimiento.
- Los tipos de petición y respuesta están definidos en un solo lugar y son
  consistentes entre servidor y futuros clientes generados desde el mismo
  archivo.

### Negativas

- Cambiar el contrato requiere regenerar antes de poder compilar. El ciclo
  de iteración sobre la API es más lento que editando código directamente.
- El proyecto depende de una herramienta externa cuya evolución no controla.
  Si oapi-codegen deja de mantenerse o cambia de forma incompatible, migrar
  implica reescribir la capa de handlers.
- El código generado arrastra dependencias que el proyecto no eligió
  directamente. Al incorporarlo, la directiva `go` del módulo subió a 1.25
  por requisito transitivo, sin que fuera una decisión del proyecto.
- La opción `embedded-spec`, activada para poder servir la documentación
  desde el binario, introduce `kin-openapi`, una dependencia considerable.
  Conviene evaluar en #17 si servir el YAML con `go:embed` cubre la
  necesidad sin ella.
- El archivo generado son más de mil quinientas líneas que aparecen en el
  diff de cualquier cambio de contrato.
- El código generado no siempre es el que se escribiría a mano, y no se
  puede ajustar: cualquier edición se pierde en la siguiente generación.

### Neutras

- La versión de oapi-codegen queda sujeta a la política del
  [ADR-0002](0002-tool-versions.md), con la duplicación que allí se
  registra: aparece tanto en `.tool-versions` como en el workflow de CI.

## Revisión

Reconsiderar si el contrato se estabiliza al punto de que el paso de
generación pese más que el beneficio, o si oapi-codegen deja de recibir
mantenimiento. En ese escenario, la alternativa a evaluar primero no es
volver a code-first sino congelar el código generado y mantenerlo a mano,
conservando la especificación como documentación verificada por pruebas de
contrato.
