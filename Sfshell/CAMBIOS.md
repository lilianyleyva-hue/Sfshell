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
