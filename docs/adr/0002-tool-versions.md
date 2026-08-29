# ADR-0002: Fijar versiones exactas de las herramientas de desarrollo

- **Estado:** aceptado
- **Fecha:** 2026-08-29
- **Issue relacionado:** #5

## Contexto

El proyecto depende de herramientas externas para compilar y verificar el
código: el compilador de Go y golangci-lint, a las que se sumarán
generadores de código y clientes de infraestructura.

Estas herramientas se ejecutan en tres entornos distintos: dos máquinas de
desarrollo y los runners de GitHub Actions. Nada garantiza que los tres
resuelvan la misma versión salvo que se declare explícitamente.

Durante la construcción del primer milestone, la ausencia de versiones
fijadas produjo tres fallos:

1. El workflow declaraba `golangci-lint-action@v6`, que instala la serie 1.x
   del linter, mientras el archivo `.golangci.yml` usaba el formato de
   configuración de la serie 2.x. El CI falló con un error de validación de
   esquema que no apuntaba a la causa real.
2. Al corregir el archivo de configuración para la serie 1.x, el entorno
   local —que tenía instalada la 2.12.2— pasó a rechazarlo. El mismo archivo
   no podía satisfacer a ambos entornos simultáneamente.
3. Una de las máquinas de desarrollo tenía Go instalado desde los
   repositorios de la distribución, en una versión anterior a la declarada en
   `go.mod`. El comando `go` intentó descargar el toolchain faltante y falló,
   dejando el entorno inoperante hasta reinstalar el compilador desde la
   fuente oficial.

El patrón común es que la versión del binario y el formato de su archivo de
configuración constituyen un solo contrato. Cambiar uno sin el otro rompe la
compilación, y el mensaje de error rara vez señala la discrepancia de
versiones como causa.

## Decisión

Las versiones de todas las herramientas de desarrollo se declaran de forma
exacta, sin rangos ni referencias móviles como `latest`.

Las versiones vigentes se registran en `.tool-versions`, que actúa como
fuente de verdad legible tanto por gestores de versiones compatibles con ese
formato como por una persona que llega al proyecto.

Los mismos valores exactos se replican en los workflows de CI, en la imagen
base del Dockerfile y en la directiva `go` de `go.mod`. Cualquier
actualización modifica todos esos puntos en un mismo commit.

Las herramientas de lenguaje no se instalan desde los repositorios de la
distribución, porque su versión y su calendario de actualización no están
bajo control del proyecto.

## Alternativas consideradas

### Referencias móviles

Usar `latest` en los workflows y aceptar la versión que provea cada entorno.
Es lo que estaba implícitamente en vigor y lo que produjo los tres fallos
descritos.

El costo real de este enfoque no es que las herramientas cambien, sino que
cambian en momentos que el proyecto no elige: una publicación del linter
puede romper un pull request que no modificó una sola línea de código, y el
diagnóstico obliga a descartar primero el cambio propio.

### Rangos de versión compatibles

Fijar la versión menor y permitir parches, por ejemplo `v2.12.x`. Reduce la
superficie del problema sin eliminarlo, y a cambio introduce ambigüedad sobre
qué se está ejecutando exactamente en cada entorno.

Se descartó porque el beneficio —recibir correcciones sin intervención— es
menor que la certeza de reproducibilidad en un proyecto de este tamaño.

### Ejecutar las herramientas en contenedores

Encapsular cada herramienta en una imagen con su versión fijada, eliminando
la dependencia del entorno local. Es la solución más robusta y la que se
adoptaría en un equipo grande.

Se descartó por ahora porque agrega una capa de indirección a todos los
comandos de desarrollo, y porque el tiempo de arranque de un contenedor por
invocación degrada el ciclo de retroalimentación de forma perceptible.

## Consecuencias

### Positivas

- Los tres entornos ejecutan binarios idénticos, de modo que un fallo de
  verificación en CI es reproducible localmente.
- Las actualizaciones de herramientas ocurren en commits deliberados, con su
  propio pull request y su propio diagnóstico si algo se rompe.
- El archivo `.tool-versions` documenta las expectativas del proyecto sobre
  su entorno, información que de otro modo vive únicamente en la memoria de
  quien lo configuró.

### Negativas

- La misma versión aparece declarada en cuatro lugares: `.tool-versions`, el
  workflow de CI, el Dockerfile y `go.mod`. Nada impide que se
  desincronicen, y la única defensa actual es la disciplina al actualizar.
  Esta es la consecuencia más seria de la decisión.
- El proyecto no recibe correcciones de seguridad de sus herramientas hasta
  que alguien actualiza las versiones manualmente.
- Configurar una máquina nueva requiere instalar versiones específicas en
  lugar de usar el gestor de paquetes del sistema, lo que hace el proceso más
  largo.

### Neutras

- La decisión aplica a las herramientas de desarrollo, no a las dependencias
  del módulo Go, que ya tienen su propio mecanismo de fijación en `go.sum`.

## Revisión

Reconsiderar si el número de herramientas crece al punto de que mantener las
declaraciones sincronizadas a mano deje de ser viable. En ese escenario, la
alternativa a evaluar primero es generar los valores de los workflows y del
Dockerfile a partir de `.tool-versions`, eliminando la duplicación en lugar
de gestionarla.
