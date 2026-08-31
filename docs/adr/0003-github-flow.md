# ADR-0003: Adoptar GitHub Flow con despliegue continuo

- **Estado:** aceptado
- **Fecha:** 2026-08-31
- **Issue relacionado:** #6

## Contexto

El proyecto necesita una estrategia de ramas y una definición de cuándo el
código llega al entorno desplegado.

Las condiciones que lo enmarcan:

- Un solo autor, sin necesidad de coordinar trabajo concurrente sobre las
  mismas áreas del código.
- Un único entorno desplegado. No existe staging ni preproducción, y no hay
  usuarios cuya operación dependa del servicio.
- No se da soporte a versiones anteriores: nadie está ejecutando una versión
  distinta de la última.
- El repositorio cumple una función de comunicación además de alojar el
  código. El historial y los pull requests son parte de lo que el proyecto
  muestra.

Esta última condición altera el cálculo habitual: un proceso que en un
proyecto personal sería ceremonia innecesaria aquí tiene valor, porque deja
registro del razonamiento detrás de cada cambio.

## Decisión

Se adopta GitHub Flow: una rama `main` protegida que debe permanecer
desplegable en todo momento, y ramas de corta duración para cada unidad de
trabajo, integradas mediante pull request con squash.

Cada integración a `main` despliega automáticamente. **Desplegar no es
publicar un release.** Un release agrupa varios cambios en un hito
describible en una frase, se marca con una etiqueta semántica y genera notas
a partir de los commits.

Esta separación permite integrar de forma continua sin que el número de
versión pierda significado. Una etiqueta por cada issue resuelto produciría
versiones que no comunican nada; una etiqueta por hito produce un registro
legible de la evolución del sistema.

Las ramas se nombran `<tipo>/<numero-issue>-<slug>` y los commits siguen
Conventional Commits, de modo que las notas de release puedan generarse
automáticamente.

## Alternativas consideradas

### GitFlow

Ramas `develop`, `release/*` y `hotfix/*` además de `main`. Está diseñado
para equipos que publican versiones con calendario y mantienen varias en
paralelo.

Se descartó porque ninguna de las condiciones que lo justifican está
presente: no hay releases calendarizados, no hay versiones simultáneas en
uso, y no hay coordinación entre desarrolladores que resolver. Adoptarlo
sería replicar la forma de un proceso sin su función.

### Integración directa a `main` sin pull requests

Con un solo autor, el pull request no cumple su propósito habitual de
revisión por pares. Habría sido defendible y más rápido.

Se descartó por la condición de comunicación descrita en el contexto: el
pull request es donde queda documentado el porqué de cada cambio, enlazado
de forma permanente al código que lo introdujo. El mensaje de commit sirve a
quien hace `git blame` dentro de dos años; la descripción del pull request
sirve a quien quiere entender la decisión completa. Son audiencias distintas
y ambas importan aquí.

### Merge sin squash

Preservar los commits intermedios de cada rama en `main`. Mantendría visible
la iteración real, incluidos los intentos fallidos.

Se descartó porque esa iteración ya queda visible dentro del pull request,
donde tiene contexto. En `main`, los commits de trabajo en curso mezclados
con los definitivos degradan la legibilidad del historial, que es
precisamente lo que se busca preservar.

## Consecuencias

### Positivas

- El intervalo entre escribir código y verlo desplegado se mide en minutos,
  lo que hace que los fallos de despliegue se detecten cerca de su causa.
- El historial de `main` contiene un commit por unidad de trabajo, legible
  de forma lineal.
- Las ramas de corta duración obligan a dimensionar los issues de manera que
  quepan en pocos días de trabajo. Una rama que se extiende es señal de que
  el issue estaba mal cortado.

### Negativas

- `main` puede quedar rota si el CI no cubre lo que el cambio introduce. La
  única defensa es la cobertura de las verificaciones automáticas, y ninguna
  cobertura es completa. Sin un entorno intermedio, un fallo que el CI no
  detecte llega directamente al servicio desplegado.
- No hay forma de integrar trabajo incompleto sin exponerlo. Implementar una
  funcionalidad que abarque varios días requiere o bien mantener una rama
  larga, contradiciendo la estrategia, o bien introducir feature flags, que
  el proyecto todavía no tiene.
- El squash descarta los commits intermedios de forma irreversible una vez
  que la rama se elimina. Si el pull request se borrara, esa iteración se
  perdería.
- La rama por defecto protegida impide corregir sobre la marcha: incluso un
  cambio de una línea requiere abrir un pull request. Es una fricción
  aceptada deliberadamente.

### Neutras

- El versionado es único para todo el sistema, coherente con la decisión de
  monorepo registrada en [ADR-0001](0001-monorepo.md).

## Revisión

Reconsiderar si aparece un segundo colaborador, momento en que el pull
request pasaría a cumplir su función de revisión y convendría exigir
aprobación. También si el proyecto llega a tener usuarios reales, caso en
que la ausencia de un entorno intermedio dejaría de ser aceptable y habría
que introducir despliegues progresivos o feature flags antes de seguir
integrando de forma continua.
