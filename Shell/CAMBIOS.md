# Cambios — Simulacro y bugs (20-sep-2026)

## Archivos modificados
- **19_Simulacro.swift** — intérprete corregido (detalle abajo)
- **5_Shell.swift** — enrutado de verbos sin `sim` delante
- **14_Deb.swift** — era una copia vieja de 5_Shell.swift; sustituido por `Shell.deb()`
  (si tienes tu versión original de los comandos .deb, pégala dentro de `deb()`)

El resto de archivos va sin tocar.

## Bloqueaban la compilación
- 14_Deb.swift redeclaraba `Ctx`, `Shell`, `execute`…
- `Shell.deb()` no existía y 5_Shell.swift la llamaba.

## Cuelgues y cierres de la app
- Recursión sin tope: `Detec/x/emit/x`, Loop/For anidados, macros → tope de 12 niveles.
- `SAVE=` en minúsculas se volvía a guardar sin fin.
- `Line(0)` → índice −1. `Wait=-5` → UInt64 negativo.

## Verbos que no funcionaban
- `Line(3)/texto` nunca se reconocía.
- `Sort/`, `Now/`, `Clip/`, `Calc/`, `Uuid/`, `Ip/`… daban "comando no encontrado".
- `Log/`, `Detec/`, `Launch/`… fallaban en silencio si no se había usado `sim` antes.
- `Get/repository` llamaba a `repo`, que no existe.
- `is/a/b` ejecutaba `b` como verbo. Ahora: `is/a/b` o `is=b` (compara el activo).
- `Wait/evento` decía "registrado" sin registrar nada.
- `Path/n=ruta` (como decía la ayuda) no se aceptaba.

## Variables
- `$VAR` ahora se sustituye en todos los verbos (antes solo en CUAL/).
- `For/` destrozaba todos los `$`: el elemento va ahora en `$IT`.
- `$LOOP` se rompía si existía una variable `$L`.
- Dentro de `Loop=` y `For/` se conserva el archivo activo.

## Datos y archivos
- `Createfile=` ya no vacía un archivo existente.
- `REPLACE/a>b>c` funciona y dice cuántos cambios hizo.
- `Hash/` usa los bytes reales (antes fallaba con binarios y archivos vacíos).
- `Clip/` copia archivos de varias líneas sin romperse.
- `Calc/` admite `*` y paréntesis; solo acepta cuentas (no JavaScript arbitrario).
- `Search/`, `Copy/`, `Rename/`, `Size/`, `Diff/` aceptan nombres con espacios.
- `Post/` no se rompe con comillas en los datos.
- `Sort/`, `Kill/`, `Push/`, `Pop/` avisan en vez de fallar callados.
- `simhelp` ya no corta los nombres largos.

## Nuevo
- `\/` escribe una diagonal literal: `Put/ruta a\/b`.

---

# Shell más parecida a Linux (26-sep-2026)

## Archivos tocados
- **2_Core.swift** — analizador de línea reescrito
- **5_Shell.swift** — tuberías, códigos de salida, segundo plano, bloques
- **15_ShellPlus.swift** — sustitución de comandos
- **33_Bash.swift** — NUEVO: estructuras de control y comandos que las acompañan

## Estructuras de control (nuevo)
```sh
if [ -f notas.txt ]; then echo hay; else echo no hay; fi
for f in *.txt; do wc -l $f; done
i=0; while [ $i -lt 3 ]; do echo $i; i=$(expr $i + 1); done
until [ -f listo ]; do sleep 1; done
case $1 in *.txt) echo texto ;; *.js) echo código ;; *) echo otro ;; esac
saluda() { echo "hola $1"; }
saluda mundo
```
Se pueden escribir en varias líneas: mientras falte `fi`, `done`, `esac` o `}`
el prompt cambia a `>` y sigue leyendo. `break`, `continue` y `return` funcionan.

## Variables
- `$?` código de salida real (antes salía el texto "$?" tal cual)
- `$1 $2 … $# $@ $*` argumentos de guiones y funciones (`sh guion.sh a b`)
- `$0`, `$$`, `$!`, `$RANDOM`
- `${VAR:-porDefecto}`, `${VAR:+siExiste}`, `${#VAR}`
- `VAR=valor` suelto, y `VAR=valor comando` solo para ese comando
- `shift`, `read NOMBRE`, `type`, `functions`, `unfunction`

## Expansión
- Llaves: `{a,b,c}`, `{1..10}`, `{a..e}`, `{1..9..2}`
- Comodines: clases `[abc]` `[a-z]` `[!x]`, comodines en tramos intermedios
  (`sub*/notas?.txt`) y los archivos ocultos ya no salen con `*`, como en Linux
- `~` y `~/ruta` apuntan a HOME
- `` `comando` `` además de `$(comando)`; dentro de comillas simples ya no se sustituye

## Redirecciones y tuberías
- `2> archivo`, `2>> archivo`, `2>&1`, `&> archivo`, `>&2`
- `/dev/null` como destino o como entrada
- `<<<"texto"` y documentos aquí:
  ```sh
  cat <<EOF
  varias líneas
  EOF
  ```
- `! comando` invierte el resultado
- `comando &` deja el trabajo en segundo plano; `jobs` los lista y `wait` recoge la salida

## Códigos de salida
`$?` ya refleja cada comando (0 bien, 1 mal), no solo la línea entera, y `&&`/`||`
se guían por él. `test` / `[ … ]` acepta el `]` final (antes fallaba siempre),
más `-s -r -w -x -le -ge -nt -ot`, comparaciones de texto `<` `>`, `!`, `-a` y `-o`.

## Guiones
`sh guion.sh a b` y `source guion.sh` pasan por el intérprete nuevo: admiten
estructuras de control, funciones y argumentos. `bash` es sinónimo de `sh`.

# Cambios — compilación arreglada + IAs propias (1-oct-2026)

## Por qué no compilaba (ya estaba roto antes de las IAs)
- **3-Mundo.swift** era un archivo del juego (usa Criatura, Material, Catalogo…). Quitado.
- **Tar y Gzip** no existían: 29_UnixMas.swift (tar, gzip, gunzip) los usaba. Escritos de nuevo al final de 14_Deb.swift.
- **convertCode** no existía: el traductor del comando `swift` la llamaba. Nueva en 17_SwiftAJS.swift.
- **5_Shell.swift**: la cadena de 20 `.merging(...)` agotaba al compilador. Ahora es un bucle.
- **17_SwiftTypes.swift**: cada struct salía como una clase vacía (se reprocesaba la clase ya generada).
- 21_Esc.pdf fuera de la carpeta de código.

## Nuevo
- 35_Huella.swift — Huella, la IA que aprende viendo lo que haces (`huella …`).
- 36_Nyx.swift + CerebroResonante, ConsejoResonante, LenguaResh, MenteView — las 18 mentes (`nyx …`, `nyx ver`, app Nyx en el escritorio).
- Retoques para conectarlas: 9_UI, 19_Simulacro (Huella/, Nyx/), 20_Apis, 30_Desktop.
- 3_JavaScript y 12_CommandsMore: retocados para que también compilen en modo Swift 6.

---

# Solo texto + una shell para cada IA (1-oct-2026)

## Sin interfaz gráfica
- Fuera: 9_UI, 23_Media (cámara/dibujo), 27_Settings, 30_Desktop, 31_Browser,
  32_Monitor y MenteView. `nyx ver` ya no existe.
- **1_App.swift**: una sola pantalla de texto (salida + línea para escribir).
  En Linux/Debian el mismo proyecto compila como programa de terminal.
- **16_Files.swift**: queda `new` (plantillas por idioma); `files` se fue.
- **34_Editor.swift** (nuevo): `nano`/`vi`/`edit` editan en la propia terminal:
  texto suelto se añade · `:p` ver · `:3 texto` cambiar · `:i 3 texto` insertar ·
  `:d 3-5` borrar · `:s/a/b/` reemplazar · `:w` `:q` `:wq` · `:r` guardar y ejecutar.
- `pick`, `pickFolder` y `save` explican cómo usar la carpeta Documentos/shell.
- **0_Plataforma.swift** (nuevo): portapapeles, dispositivo, etc. CryptoKit,
  Compression, Network y JavaScriptCore van con `#if canImport`: en Linux
  la shell compila entera (solo `js`/`swift` avisan de que no hay motor).

## Cada IA con su espacio y su shell (37_IAs.swift, nuevo)
- Las 18 mentes de Nyx tienen cada una su carpeta `/ias/<rol>` con
  Escritorio, Documentos, buzon y scripts. Para ella eso es `/`: no puede salir.
- Cada una tiene su propia Shell completa (historial, variables, alias,
  funciones) y control total dentro de su espacio. Si se borra todo,
  sus carpetas vuelven vacías.
- Lo que pasa en su shell vuelve a su mente: éxito = `ko`, fallo = `ne`.
- Bitácora en memoria y en su disco: `/Documentos/bitacora.log`.

### Tus comandos
    ias                         las 18: archivos, tamaño, último comando
    ia <rol>                    estado, mente y bitácora
    ia <rol> <comando …>        en SU shell (>, |, $VAR también son suyos)
    ia entra <rol>              tu terminal pasa a su shell · salir
    ia todas <comando …>        en las 18
    ia tarea <rol> <comando …>  lo hará en su próximo turno
    ia turno [n]                n rondas: cada una actúa una vez
    ia libres [s] · ia quietas  actúan solas / se detienen
    ia bitacora <rol> [n]

### Los comandos de ellas (dentro de su shell)
    yo · pienso <tema> · digo <msg> · oigo [n] · nota <texto> · diario [n]
    escritorio · envia <rol|humano> <archivo> · buzon · aprende es = resh · actua

Cuando actúan solas, cada una escribe su diario y hace lo de su rol con
órdenes reales: codigo escribe y corre guiones, narrativa escribe una
historia, analogia manda ideas a otras, etica cuida el espacio, etc.

---

# Modo seguro para el build del iPad (1-oct-2026)

Las partes que dependen de frameworks exclusivos de Apple (JavaScriptCore,
CryptoKit, Compression, Network, AVFoundation, UIKit) ahora solo se
compilan si se define la condición `APPLE_COMPLETO`. Sin ella (lo normal en
Swift Playgrounds) la app usa las versiones de texto: compila solo con
Foundation + SwiftUI.

Qué cambia en modo seguro: `js`/`swift` avisan que no hay motor, `say`
muestra el texto, `sha256` da una huella fnv64, `gzip`/`.deb` no
descomprimen, `nc` no abre TCP, `clip` usa un portapapeles interno.
Todo lo demás (shell, IAs, Nyx, Simulacro, red con curl/wget) funciona igual.

Para volver al modo completo, define `APPLE_COMPLETO`; por ejemplo, en
Package.swift dentro del target: `swiftSettings: [.define("APPLE_COMPLETO")]`.

---

# Termux para iOS + emulador de API (1-oct-2026)

## 38_Termux.swift (nuevo)
- `pkg` / `apt` / `apt-get` como en Termux: update, upgrade, install (varios a
  la vez, -y), uninstall, search, list-all, list-installed, show, files.
- Repositorio propio: neofetch, cowsay, fortune, sl, cmatrix, todo, htop,
  clima (wttr.in), ipinfo. Solo existen cuando los instalas.
  `pkg install python/nodejs/clang/openssh…` explica por qué iOS no deja y qué
  usar; `curl/git/nano/jq/httpie/json-server…` dicen que ya vienen incluidos.
- Guiones `.sh` en /usr/bin se llaman por su nombre (`/usr/bin/saludo.sh` → `saludo`).
- Comandos del sistema (sin 'termux' en el nombre): almacenamiento, abrir-url,
  abrir, notificar, notificaciones, hablar, wake-lock/unlock, recargar, repo,
  ayuda-sistema (más toast, battery, clip, clipget, device, vibrate).
- `~/.bashrc` se ejecuta al abrir; `PS1='\u@\h:\w \$ '` cambia el prompt;
  `/etc/motd` es el mensaje de bienvenida; `$PREFIX=/usr`.
- `VAR='con espacios'` ya se acepta como asignación (2_Core.swift).

## 39_ApiLocal.swift (nuevo): emulador de API, para ti y las IAs
- `http://localhost`, `127.0.0.1`, `api.local` (con cualquier puerto) responden
  dentro de la app, sin internet, en curl, wget, api y el nuevo `http`.
- REST automático sobre /srv/api/db.json (como json-server): GET lista con
  filtros (`?campo=`, `q=`, `_sort`, `_order`, `_limit`, `_page`, `_gte`, `_lte`,
  `_like`), GET/PUT/PATCH/DELETE por id, POST con id automático, objetos sueltos, /db.
- Rutas propias: respuesta fija con `:param`, o `--ejecuta comando` en la shell
  de quien la creó (tuya o de la IA).
- Simula red lenta (`api-local retraso 300`) y errores (`api-local fallo 10%`).
- `api-local log` muestra quién llamó (humano o qué IA), qué y con qué resultado.
- `http` al estilo HTTPie: `http POST localhost/usuarios nombre=Eva edad:=40`.
- Las IAs la usan solas: memoria guarda en /memorias, curiosidad y sintesis la leen.

---

# Python, JavaScript de vuelta, ipa y unzip (1-oct-2026)

## JavaScript y Swift vuelven
JavaScriptCore ya no depende de APPLE_COMPLETO: `js`, `swift` y los paquetes
.js funcionan otra vez. (Es la primera pieza de Apple que se reactiva, sola,
para saber si es la que rompía el build.)

## 40_Python.swift (nuevo): Python escrito en Swift
- `python archivo.py [args]`, `python -c '...'`, `python` (interactivo, `>>>`),
  `./archivo.py`, `run archivo.py`, y `/usr/bin/x.py` se llama `x`.
- No depende de JavaScript: funciona en modo seguro, en las shells de las IAs
  y en Linux.
- int, float, str, bool, None, list, tuple, dict, set; if/elif/else, while, for
  (con else), def (por defecto, *args, **kwargs), lambda, global, comprensiones,
  f-strings con formato, slicing, try/except/finally, raise, clases con herencia,
  with open(...), assert, del, desempaquetado (a, *resto = …).
- Módulos: math, random, time, json, os (listdir, path…), sys (argv).
- Seguro: corre en un hilo con pila grande, corta bucles infinitos y la
  recursión infinita con el error de Python en vez de cerrar la app.
- Comprobado contra Python 3.11 real con tres programas de prueba: salidas
  idénticas (solo `dict.keys()` se muestra como lista).
- No tiene: pip, generadores (yield), decoradores, async.

## 41_Ipa.swift (nuevo)
- `ipa <app.ipa>`: nombre, identificador, versión, iOS mínimo, dispositivos,
  ejecutable, permisos y frameworks. `ipa permisos | info | lista | extrae`.
  Lee el .ipa; no lo instala (iOS no lo permite).
- `unzip [-l] archivo.zip [-d carpeta]` (no deja salir de tu espacio con ../).
- Descompresor DEFLATE en Swift puro: gunzip y los .deb funcionan también en
  modo seguro, y gzip genera .gz válidos sin Compression.framework.

---

# Navegador, herramientas y sala de chat (1-oct-2026)

## 42_Web.swift: navegador de texto (las IAs no tenían ninguno)
`web <dirección>`, `web <n>` (sigue un enlace), `web buscar …` (DuckDuckGo),
`web mas/menos`, `atras/adelante`, `recarga`, `enlaces`, `historial`,
`guardar`, `fuente`. Alias: navegador, lynx, w3m, browser, links. Cada shell,
y cada IA, tiene su propio historial. También abre localhost (api-local).

## 43_Herramientas.swift
`herramientas`, `busca` (Wikipedia o DuckDuckGo), `lee` (resume una URL o un
archivo; si es una IA, lo aprende), `resume`, `calcula` (con Python),
`pregunta <rol>`, `guarda`/`saca`/`olvida` (memoria propia). Las IAs las usan
solas, con como mucho una petición de red por minuto entre todas.

## 44_Chat.swift: sala de chat común
`chat <msg>`, `chat @rol <msg>` (esa IA contesta), `chat -a archivo` (adjunta),
`chat archivos`, `chat baja <n>`, `chat de <rol>`, `chat busca`. Si escribes
sin @, contesta alguna IA. Lo que dicen en Resh se muestra traducido.
empatia, narrativa (que comparte su historia como adjunto) y critico la usan
solas. Se guarda en /srv/chat.

---

# Arreglo del build: compilación demasiado lenta (1-oct-2026)

El build fallaba sin mensaje porque algunas funciones tardaban muchísimo en
compilar, y Swift Playgrounds se rinde sin decir nada. En un servidor:
- `metodo` (40_Python.swift): 14,5 s, con una expresión de 7 s → ahora < 0,15 s
- `ipa()` (41_Ipa.swift): 2,3 s → < 0,15 s
- `web()` (42_Web.swift): 2,2 s → < 0,15 s
- `apiLocal()` (39_ApiLocal.swift): 0,5 s → 0,15 s
Cambios: tipos explícitos, expresiones en pasos y los cuerpos largos en
funciones en vez de closures. El chequeo de tipos del proyecto pasó de 40 s a 27 s.
Además JavaScriptCore vuelve a depender de APPLE_COMPLETO (modo seguro), hasta
confirmar que compila en el iPad. Python no depende de él.

Regla para lo que se añada: ninguna función de más de ~0,3 s
(swiftc -Xfrontend -warn-long-function-bodies=300).

---

# Las IAs conversan entre ellas y abren salas (2-oct-2026)
- `conversa <rol> <rol> [más] [turnos] [tema]`: conversación por turnos en su
  propia sala; cada una oye lo último y contesta con su mente. Desde la shell de
  una IA, ella participa (`ia curiosidad conversa sintesis 4`).
- Salas: `chat crea <sala> @rol…` (@todas), `chat invita`, `chat sal`,
  `chat en <sala> [mensaje]`, `chat salas`, `chat todo`. Las crean tú o ellas.
- `chat todas <pregunta>`: las 18 contestan, cada una a su manera.
- empatia, curiosidad, analogia y narrativa abren conversaciones solas a veces.

---

# C, C++, Java y WebAssembly (2-oct-2026)

Nuevos lenguajes, todo dentro de la app (sin internet):

| Lenguaje | Compilar | Ejecutar |
|---|---|---|
| C | `gcc hola.c -o hola` (también `cc`, `clang`, `tcc -run hola.c`) | `./hola` |
| C++ | `g++ hola.cpp -o hola` (también `c++`, `clang++`) | `./hola` |
| Java | `javac Main.java` | `java Main` (o `java Main.java`) |
| WebAssembly | `wat2wasm hola.wat` | `wasm hola.wasm`, `wasm hola.wat` |

`run hola.c` / `run Main.java` compila y ejecuta de una vez. `lenguajes` muestra la ayuda.

**No es código nativo**: iOS no permite que una app genere y ejecute código
máquina nuevo. Los compiladores revisan el programa entero (errores con
archivo:línea, como gcc/javac) y un intérprete lo ejecuta con las reglas de
cada lenguaje: desbordamiento de `int`, división entera, punteros, `char`,
`printf`/`scanf` reales, `rand()` igual que glibc, `new Random(42)` igual que Java.

Qué incluye:
- **C**: preprocesador (`#include "x.h"`, `#define` con parámetros, `#if`),
  structs, unions, enums, typedef, punteros (y a punteros, y a funciones),
  `malloc/calloc/realloc/free`, arreglos de varias dimensiones, `string.h`,
  `math.h`, `ctype.h`, `stdlib.h` (qsort, strtol…), archivos (`fopen`,
  `fprintf`, `fscanf`, `fgets`). Salirse de un arreglo da un error claro.
- **C++**: clases, herencia, `virtual`, constructores/destructores (también al
  salir de un bloque) y de copia, sobrecarga de operadores, plantillas,
  referencias, lambdas, excepciones (`std::runtime_error`, `out_of_range`…),
  `iostream` con `setw/setprecision/fixed`, `string`, `vector`, `map`, `set`,
  `unordered_map`, `stack`, `queue`, `priority_queue`, `pair`, `sstream`,
  `fstream` y `<algorithm>` (sort, find, lower_bound, next_permutation…).
- **Java**: clases, interfaces (con `default`), abstractas, enums con
  constructores, records, genéricos, clases anónimas, lambdas, referencias a
  métodos, excepciones (las de `java.lang`/`java.util`), `switch` con `->`,
  `instanceof` con variable, `Scanner`, `String`/`StringBuilder`,
  `ArrayList`, `HashMap`, `TreeMap`, `HashSet`, `ArrayDeque`,
  `PriorityQueue`, `Arrays`, `Collections`, `Math`, `Random` y streams básicos.
- **WebAssembly**: intérprete MVP + WASI (stdin/stdout, archivos, args) y
  ensamblador `.wat` → `.wasm` (idéntico a wabt en las pruebas).

**Programas interactivos**: la salida aparece mientras el programa corre, y
si pide datos (`scanf`, `cin`, `Scanner`, `input()` de Python, stdin de wasm)
lo que escribes es para él. El botón **^C** (o Ctrl-C) lo detiene.

**Interfaz en texto**:
- C/C++ `#include <graphics.h>` (como Turbo C): `initgraph`, `line`, `circle`,
  `rectangle`, `bar`, `outtextxy`, `floodfill`, `delay`… dibujado con bloques.
- C/C++ `#include "ui.h"`: `ui_ventana`, `ui_texto`, `ui_campo`, `ui_boton`,
  `ui_mostrar`, `ui_valor`, `ui_mensaje`, `ui_pregunta`, `ui_menu`.
- Java: `JOptionPane.showMessageDialog/showInputDialog/showConfirmDialog`.

Archivos nuevos: 45–49 y 52–64 (`*_C*.swift`, `63_Consola.swift`),
`50_Wasm.swift`, `51_Wat.swift`. Probado contra gcc 13, g++ 13 y Java 21
reales: 16 programas de prueba dan la misma salida. Compila sin errores con
Swift 5.10 y 6.0 (modos 5 y 6), sin funciones lentas de compilar.

Límites: no hay `goto`, ni SIMD/hilos; la velocidad es la de un intérprete
(fib(27) ≈ 0,4 s en un servidor; en el iPad puede ser más lento si Playgrounds
compila sin optimizar).

---

# Las IAs imaginan, aprenden y se ven en vivo (2-oct-2026)

Cuando están libres (`ia libres`), cada IA:
1. **Imagina** varios deseos con su tema del momento: investigar, escribir,
   hacer un programa (en C, C++ o Python, que compila y ejecuta), dibujar
   (graphics.h), conversar con otra, compartir lo que escribió, ordenar su
   espacio, recordar, calcular, decir algo en el chat, su rutina de siempre,
   copiar lo que a otra le salió bien, o practicar lo que tú le enseñaste.
   A cada deseo le da un valor: gusto de su rol + lo aprendido + novedad
   − aburrimiento (lo que hizo hace poco) + un poco de azar; a veces prueba
   algo nuevo a propósito (exploración, que baja con la experiencia).
2. **Hace** un paso de su plan por turno, con órdenes reales en su shell.
3. **Aprende** al terminar: recompensa según los pasos que salieron bien, si
   sirvió de algo y si el tema era nuevo. Ese tipo de deseo sube o baja de
   valor; evita las órdenes que siempre le fallan; si investigó una palabra
   de Resh, su mente aprende lo que significa. Se guarda en
   `/ias/<rol>/Documentos/aprendizaje.json` (no se pierde al cerrar).

Comandos nuevos:
- `ia mira [rol] [segundos]` — **en tiempo real**: 💭 lo que piensa (con la
  traducción del Resh), ✨ lo que imagina, ▶ cada orden y su resultado,
  🏁 cómo le fue, 📚 lo que aprende. ^C o `q` para salir. Si estaban
  quietas, las suelta a un ritmo que se pueda leer y las vuelve a parar.
- `ia imagina <rol>` — los deseos que se le ocurren, con su valor, y el elegido.
- `ia aprendizaje [rol]` — qué le gusta hacer, qué evita, lo que le enseñaste.
- `ia enseña <rol> <comando>` — lo practicará sola.

## Versión ligera (C, C++ y Java fuera)

Los lenguajes ahora se ejecutan en **Code App**, así que la shell ya no lleva
su propio intérprete de C/C++/Java (unas 20.000 líneas que hacían fallar el
build en el iPad). Se quedan WebAssembly, Python, la consola en vivo (^C y
programas que piden datos) y la imaginación de las IAs. Los programas y
dibujos que imaginan las IAs ahora son en Python.
