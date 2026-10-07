# Taller de Nyx y Abla

Nyx y Abla empiezan **sin nada**: sin plantillas, sin planos, sin saber qué es
una casa o un árbol, y sin el mundo de pasillos de Nyx Mundo. Solo tienen sus
idiomas y una instrucción:

> **crea un mundo infinito con lo que sabes**

Lo que dicen y lo que piensan se convierte en mundo, palabra a palabra. Con
sus **manos** cogen lo que ya existe, suyo o de la otra, y lo cambian. No hay
límite ni hacia arriba ni hacia abajo.

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

## En la ventana

| Tecla | Qué hace |
|---|---|
| **W A S D** | andar (Mayús para correr) |
| **Espacio** | saltar |
| **V** | volar (Espacio sube, C baja); otra vez V para dejar de volar |
| **E** | usar lo que tengas cerca (si una pieza tiene `AlUsar`) |
| **Y** | mostrar u ocultar la charla |

- **El vacío.** No hay suelo de partida: empiezas volando. Lo que pisas es lo
  que ellas han hecho, a la altura que sea (+90 m, −60 m o lo que sea). Si
  dejas de volar sobre algo, caes hasta ello. Si no hay nada debajo, a los
  pocos segundos vuelves a volar. Con `pasillos on` vuelve el mundo de Nyx
  Mundo de fondo.
- **Choques exactos.** La física sale de la forma real de cada cosa, medida a
  cada altura. Nada choca más de lo que mide (te acercas a un poste fino hasta
  tocarlo), por debajo de un arco se pasa y las escaleras se suben.
- **Lo que se mueve y suena.** Las piezas pueden tener partes que giran o
  suben y bajan, scripts con eventos (`Empezar`, `Actuar`, `AlUsar`,
  `AlEntrar`, `AlSalir`, `AlTocar`) y sonido en 3D: agua, fuego, viento,
  pájaros, campanas, música… Cuando una dice algo de sonido, su frase se
  vuelve melodía.

## Qué hacen solas

Mientras el taller está abierto se turnan, sin parar:

1. **Nyx** contesta con su propia cabeza, que no se toca, a lo último que dijo
   Abla. La primera vez contesta a la instrucción.
2. **Uno de los 27 de Abla** contesta a lo que dijo Nyx, en su idioma. Si no
   tiene respuesta, se oye una de las cosas que han estado pensando.
3. **Lo que dice cada una se construye.** Cada una tiene un constructor que va
   por el mundo sin límite, y cada palabra es un gesto:
   - **Dirección:** las palabras de dirección lo mueven. *Subir, arriba,
     cielo…* lo suben; *bajar, abajo, tierra, profundo…* lo bajan; *ir,
     camino, lejos…* lo llevan adelante; *girar, vuelta…* lo giran.
   - **Tamaño y color:** *grande*, *pequeño* y los colores lo cambian.
   - **Luz, suelo y sonido:** *luz* enciende una luz, *casa* o *lugar* ponen
     un suelo donde pisar y *voz* o *música* dejan su melodía.
   - **Tiempo:** *tiempo* o *vida* hacen que lo siguiente se mueva.
   - **Memoria:** *recordar* guarda dónde está el constructor y *volver*
     regresa allí.
   - **Todas las demás palabras son formas.** Cada palabra, de Abla o de Nyx,
     es siempre la misma forma, del mismo tamaño, así que su idioma se
     convierte en algo que se ve.
4. **Las manos (el editor).** Con *tomar* o *coger* agarran la pieza más
   cercana, suya o de la otra. Mientras la tienen:
   - *subir*, *bajar*, *ir* y *girar* la mueven;
   - *grande* y *pequeño* la cambian de tamaño;
   - un color la tiñe;
   - *copiar* la duplica;
   - *quitar* o *borrar* la sacan del mundo;
   - *escribir* o *editar* meten sus palabras como código dentro de ella;
   - *soltar* la deja.

Cada pieza es código Go escrito así. Si les enseñas vídeos (`ven …`), lo que
ven entra en lo que dicen y piensan, y de ahí en lo que construyen.

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
| `taller_obra.go` | las piezas, sus versiones, Blender (exportar e importar) |
| `taller_formas.go` | las herramientas nuevas para construir |
| `taller_lengua.go` | de palabras a construcción: el constructor de cada una y sus manos (el editor) |
| `taller_motor.go` | el motor: partes, scripts, eventos, sonidos, física exacta, juegos |
| `taller_motor.js` | en la ventana: caminar con física, dibujar lo que se mueve, sonido 3D |
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
