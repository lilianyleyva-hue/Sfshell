# Puerto a C++/Debian 13 — estado del proyecto

Petición original: "haz una versión de todo esto pero para Debian 13
Linux en Java, C o C++". El usuario eligió **C++** y **todo, módulo por
módulo** (entrega incremental cubriendo los 33 archivos Swift).

## Entregado (primera iteración, con extras) — `swiftshell-cpp.tar.gz`

Proyecto CMake completo en `src/core/` + `src/commands/`. Cubre:

- Núcleo: `ShellEnv`, `Parser` (tokenizer/glob/jobs), `Shell` (dispatcher,
  pipelines, redirección, `$(...)`, historial, perfil, ganchos de UI
  reimplementados como acciones reales de Debian: `nano`/`vi` de verdad,
  `xdg-open` de verdad, copiar archivos reales en vez de un picker).
- Comandos de archivos, texto y datos (json/xml/plist con parser XML y
  JSON propios, sin dependencias).
- Motor JS: QuickJS-ng vía CMake `FetchContent` (necesita red al
  configurar). Transpilador Swift→JS completo, incluyendo `convertCode()`
  que faltaba en el original (ver bugs abajo). **Ya compilado y probado
  con QuickJS real** (ver bug 5) — `js 1+1` y `swift print(1+2)`
  funcionan de verdad, confirmado por el usuario en su máquina.
- **Simulacro completo** (prioridad del proyecto: "arregla errores de
  simulacro y bugs"), con `.esc`, objetos `.eop`/mensajes, y comandos
  `.mix`. Incluye `wait{N/unidad}` y el nuevo `Npc/` (ver secciones
  propias abajo).
- `main.cpp`: REPL con readline (o entrada simple si no hay readline).

Todo el detalle de arquitectura, decisiones y bugs corregidos está en el
`README.md` dentro del tarball entregado al usuario.

## El usuario ya lo compiló en su Debian real — CONFIRMADO funcionando

Máquina real: Debian con GCC 14.2, cmake 3.31. readline/zlib/curl-dev
todos presentes y detectados bien. `FetchContent` de QuickJS-ng descarga
bien con su red. Tras el fix del bug 4 y luego el bug 5 (abajo), el
usuario compiló limpio y confirmó ("funciona ahora") — el motor JS y
todo lo demás ya corren de verdad en su máquina, no solo en este sandbox.

Durante el proceso de instalación surgieron además varios problemas que
NO eran bugs del código, sino de cómo el usuario ejecutaba los comandos
en su terminal (documentados para no repetir el diagnóstico si vuelven a
aparecer):

- Comillas sin cerrar en un `find` pegado en el terminal (posiblemente
  por autocorrección de comillas tipográficas en un teclado de
  iPad/iPhone) dejó varios comandos atrapados dentro de un solo `find`
  sin ejecutarse — solución: `Ctrl+C` y reescribir sin comillas.
- Mezclar sesiones `root` (`su`) y `mobile` repetidamente dejó archivos
  con dueños distintos en la misma carpeta, causando errores de
  `tar`/`rm`/`cmake` tipo "Cannot open: File exists" / "Permission
  denied". El usuario no sabe la contraseña de su propio `sudo`, pero sí
  la de `root` (`su` le funciona) — la solución fue hacer TODO el
  proceso de forma consistente como `root`, con rutas absolutas en vez
  de `~` (que apunta a `/root`, no a `/home/mobile`, dentro de `su`).
- Al pegar instrucciones con un comentario de ejemplo (`# contraseña de
  root`) literalmente como si fuera la propia contraseña, `su` falló su
  autenticación en silencio y ninguno de los comandos siguientes llegó a
  ejecutarse — aclarado que los comentarios `#` en los bloques de código
  son notas, no texto para pegar tal cual.

## Bugs reales encontrados y corregidos

1. `14 Deb.swift` del proyecto original está duplicado con `5
   Shell.swift` — documentado, la lógica real llega con UnixMas.
2. `3 JavaScript.swift`: `transpile()` llama a `convertCode()`, función
   que no existe en ninguno de los 33 archivos — escrita desde cero en
   `SwiftTranspiler.cpp`.
3. **Bug propio del puerto**: la expansión de `$VAR`/comodines se
   resolvía para toda una línea `cmd1; cmd2` de una sola vez, antes de
   ejecutar nada. Corregido con `Parser::splitTopLevel` +
   `Shell::runStatement` (resuelve cada tramo justo antes de ejecutarlo).
4. **Bug propio del puerto, encontrado por el usuario al compilar**:
   `CMakeLists.txt` listaba a mano los `.c` de QuickJS-ng, incluyendo
   `libbf.c` — no existe en quickjs-ng v0.10.1. Arreglado con
   `file(GLOB ...)` + exclusión por patrón.
5. **Bug propio del puerto, encontrado por el usuario al compilar**: con
   QuickJS habilitado, `make` fallaba de verdad, por dos causas:
   - `src/core/JSRuntime.cpp` incluía `quickjs.h` **dentro de
     `namespace sw`**, chocando con la propia clase `sw::JSRuntime`.
     Arreglado moviendo el include fuera del namespace.
   - `Json.hpp`, `Xml.hpp`, `JSRuntime.cpp` y `SwiftTranspiler.cpp` usan
     `std::sort` sin `#include <algorithm>` — compilaba por suerte de
     orden de inclusión hasta que QuickJS cambió ese orden. Arreglado.
   Verificado de verdad en esta sesión (con acceso real a GitHub) y
   luego **confirmado por el usuario en su propia máquina**.

## Función: `wait{N/unidad}` (pedida por el usuario)

Sintaxis `wait{25/s}` / `wait{25/m}` / `wait{25/h}` / `wait{25/d}`,
aplicable a Simulacro (`Wait/`) y a `.esc`, con espera real y techo de 1
hora. Confirmada por `AskUserQuestion` antes de implementar (la petición
original era ambigua). Implementada en `parseDuracionSegundos()`
(`Utils.hpp`), rama `wait{` en `runSim()` (`Simulacro.cpp`) y en
`corre()` (`Esc.cpp`), y en la detección de verbos sin `sim` delante
(`Shell::runStatement`, corta también en `{`). Probada: caso encadenado
(no se come verbos siguientes), caso de unidad inválida (da error, no
cuelga).

## Función nueva: `Npc/` — NPC con Q-learning real (pedida por el usuario)

El usuario pidió "un juego con un NPC con Q-learning" tras confirmar por
`AskUserQuestion` que quería un juego de persecución (NPC vs jugador),
jugable con verbos de Simulacro. Implementado en
`src/commands/NpcGame.cpp` (módulo nuevo, no existía en el proyecto
original), verbo `npc` añadido a `simVerbos`:

```
Npc/new[/N]        partida nueva (tablero NxN, 3-12, 7 por omisión)
Npc/mover/arriba    (abajo/izquierda/derecha/quieto) — juegas, el npc
                   responde con Q-learning y aprende de esa jugada
Npc/mapa            ver el tablero sin mover a nadie
Npc/entrenar/N       N episodios rápidos contra un jugador simulado
Npc/estado           episodios, capturas, tamaño de la tabla Q, α/γ/ε
Npc/resetq           borra la tabla Q (no la partida)
```

Es Q-learning tabular de verdad: estado = posición relativa
jugador/npc, 5 acciones, `Q(s,a) += α(r + γ·max Q(s',·) − Q(s,a))`,
política ε-greedy (ε=0.2), recompensa −1 por paso / +20 al atrapar. El
estado de la partida y la tabla Q se guardan en `/var/sim/npc/*.json` —
persiste entre llamadas a `swiftshell`, no solo dentro de un REPL.

**Probado de verdad, con aprendizaje real confirmado**: sin entrenar, el
npc no atrapaba al jugador simulado en 150 pasos (tope de seguridad);
tras 300 episodios el promedio bajó a ~35 pasos, tras 600 a ~23 — mejora
consistente y esperable de Q-learning. La tabla Q converge a exactamente
81 estados en un tablero 5x5 (9×9 posiciones relativas posibles), el
número exacto esperado — confirma que la representación de estados es
correcta. Probado también el flujo de captura real vía `Npc/mover`
(no solo `entrenar`): mapa marca 'X', cuenta episodios/capturas,
reinicia posiciones conservando la tabla Q. Compilado sin ningún
warning. **Pendiente: que el usuario lo pruebe en su máquina** (aún no
lo ha hecho, se le acaba de entregar).

## Mensaje del usuario "remove|type{file[name=fff.c/" — descartado

Sintaxis que no corresponde a ningún verbo real de Simulacro. Preguntado
por `AskUserQuestion`; el usuario respondió "olvidalo". No se implementó
nada — no retomar salvo que lo mencione de nuevo.

## Pendiente para la siguiente entrega (registrado vacío en
`StubCommands.cpp`, no como comandos falsos)

Red avanzada (nc/telnet/ping/whois), gestor de paquetes `pkg`, `git`
local, `apidef`/sysapi, listado de paquetes al estilo apk, UnixMas
(tar/gzip real con zlib, ifconfig, ssh/scp/sftp reales), el juego
`adivina`, y los extras de "26 MacOS.swift" (pbcopy/pbpaste/say
adaptados a xclip/xsel/espeak-ng). Fuera de alcance total:
"3-Mundo.swift" (juego 3D sin relación) y las pantallas gráficas
(Desktop/Browser-WebView/Monitor/Settings), que necesitarían GTK/Qt.

## Próximo paso

1. Que el usuario pruebe `Npc/` en su máquina (compilar con el tarball
   más reciente, jugar unas rondas, entrenar, ver `Npc/estado`).
2. Seguir módulo por módulo: red avanzada + paquetes, luego
   git/APIs/apk, luego UnixMas + logic + MacOS-extras, cada vez
   compilando y probando igual que en esta entrega antes de mandar el
   siguiente paquete.
