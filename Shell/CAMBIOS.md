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
- `termux-*`: setup-storage, open-url, open, toast, notification(+list),
  tts-speak, wake-lock/unlock, reload-settings, change-repo, help.
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
