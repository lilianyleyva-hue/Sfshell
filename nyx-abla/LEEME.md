# Taller de Nyx y Abla

Nyx y Abla ven vídeos de YouTube juntas y construyen un mundo en 3D con lo
que recuerdan. Las dos pueden crear cosas y las dos pueden cambiar lo que hizo
la otra.

- **Nyx no cambia.** Estos archivos se **añaden** a la carpeta de Nyx
  (`taller_*.go`). No modifican ninguna línea suya: ni su cabeza (`mente.go`),
  ni Nyx Mundo. El taller le habla a Nyx como le hablarías tú.
- **Abla se queda como está.** Es el programa en C de `../especie`, con sus 27
  seres, su idioma de 4000 palabras y su pensamiento sin parar. Corre a la
  vez que Nyx, en *modo puente* (`abla --puente`), y guarda su memoria aparte,
  en `~/.local/share/nyx-mundo/abla/`.
- **Todo es local.** Corre en tu ordenador y guarda todo en tus archivos. Solo
  se conecta a internet para bajar los vídeos que les enseñes.

## Instalar (Debian 13)

```sh
sudo apt install golang-go build-essential ffmpeg yt-dlp
sh nyx-abla/instalar-abla.sh ~/nyx-go      # la carpeta donde tienes Nyx
```

Si YouTube no deja bajar los vídeos con el `yt-dlp` de apt, instala uno más
nuevo con `pipx install yt-dlp`.

## Usar

```text
nyx taller            (o nyx-taller)
taller› camina                                  abre el mundo y las ves trabajar
taller› ven https://www.youtube.com/watch?v=…   que lo vean las dos
taller› charla                                  lo que se han dicho
taller› obra                                    lo que han construido
```

En la ventana del mundo, la tecla **Y** muestra u oculta la charla. Desde la
ventana también puedes escribirles a las dos.

## Qué hacen solas

Mientras el taller está abierto trabajan por turnos, sin parar:

1. **Abla** cuenta en su idioma lo que vio en cada fotograma y construye algo
   con eso: sus colores reales, sus bordes y los conceptos que le evocó. Cada
   tribu trabaja a su manera:
   - **lenguaje** hace estelas con frases en Abla;
   - **data** hace murales en relieve con la foto misma;
   - **lógica** hace fractales, espirales y escaleras.
2. **Nyx** construye con lo que recuerda de los sitios que vio: los colores de
   paredes, suelo y techo; si había cielo, agua, plantas u oscuridad; y las
   cosas que recortó de lo que vio, que pone en estatuas. Le cuenta a Abla lo
   que hizo y uno de los 27 le contesta.
3. **Cada una retoca lo de la otra.**
   - Abla le pone anillos de luz, le escribe en su idioma, la tiñe o le cambia
     el tamaño.
   - Nyx le pone encima cosas que vio, le hace un suelo o la tiñe con los
     colores de sus recuerdos.
4. Algunos de los 27 de Abla **salen a caminar** por el mundo de Nyx, como
   entidades, diciendo lo último que han pensado.
5. A veces Abla le pide a Nyx una criatura, y Nyx la escribe a su manera.

## Código y 3D

Cada pieza es un programa en Go: `~/.local/share/nyx-mundo/objetos/<pieza>.go`.

- Junto a cada pieza hay un modelo para Blender, `<pieza>.obj` + `<pieza>.mtl`,
  con sus colores. Lo que brilla va como material emisivo.
- Cuando alguien cambia una pieza, sale una versión nueva (`-v2`, `-v3`…) y
  la anterior se queda guardada.

| Orden | Qué hace |
|---|---|
| `exporta` | Todo el mundo en una escena: `taller/blender/mundo.obj`. |
| `importa archivo.obj [alto 3] [como nombre]` | Trae un modelo tuyo de Blender, con sus colores y materiales. Si tiene más de 30 000 triángulos, se simplifica. |
| `nueva mi_pieza.go` | Pone tu propio código como pieza. |
| `codigo <pieza>` | Enseña el código de una pieza. Después de editar el archivo, `recompila <pieza>`. |
| `formas` | Todo lo que se puede usar para construir. |

Con `formas` verás que, además de lo que ya tenía Nyx Mundo (caja, esfera, luz,
tubo, figura), hay:

- cilindro, cono y toro;
- revolución (tornear un perfil) y extrusión de polígonos;
- relieves y letras en 3D;
- copiar, mover, girar y escalar partes;
- color por HSV y ruido.

Todo eso también sirve en las entidades.

## Archivos

| Archivo | Para qué |
|---|---|
| `taller_main.go` | `nyx taller`: la consola y la ventana con la charla |
| `taller_charla.go` | los turnos, la conversación y ver vídeos juntas |
| `taller_ideas.go` | cómo escriben código Nyx y Abla, y cómo retocan lo de la otra |
| `taller_obra.go` | las piezas, sus versiones, Blender (exportar e importar) |
| `taller_formas.go` | las herramientas nuevas para construir |
| `taller_abla.go` | el puente con Abla (el programa en C) |
| `instalar-abla.sh` | compila Abla, añade el taller a Nyx y recompila |

Lo que guardan, en `~/.local/share/nyx-mundo/`:

| Ruta | Qué hay |
|---|---|
| `taller/obra.json` | las piezas, quién las hizo y quién las cambió |
| `taller/charla.txt` | todo lo que se han dicho |
| `taller/videos/` | los vídeos que vieron |
| `abla/` | la memoria de Abla |
| `abla/fotos/vistas.jsonl` | lo que vio en cada fotograma |
