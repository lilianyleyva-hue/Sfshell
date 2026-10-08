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
4. **Más palabras que entienden**:
   - **Números** (*dos*, *tres*… o *5*): cuántas de lo siguiente.
   - **Colocación:** *encima*, *debajo*, *al lado*, *dentro* o *alrededor* de
     lo último que pusieron.
   - **Materiales:** *madera*, *piedra*, *ladrillo*, *metal*, *hierba*,
     *mármol*, *cristal*, *neón*… (con un color delante, teñido).
   - **Texturas propias:** *textura <palabra>* (cada palabra es un dibujo
     distinto).
   - **Formas propias:** *llamar <palabra>* guarda lo que acaban de hacer como
     forma con ese nombre. Desde entonces, decir esa palabra la vuelve a
     poner.
5. **Las manos (el editor).** Con *tomar* o *coger* agarran lo más cercano,
   o *coger <nombre>* lo que se llame así, suyo o de la otra. Mientras lo
   tienen:
   - *subir*, *bajar*, *ir*, *al lado* y *girar* lo mueven; *inclinar* lo
     ladea;
   - *grande* y *pequeño* cambian su tamaño;
   - un color lo tiñe, un material lo pinta y *textura <palabra>* le pone
     esa textura;
   - *encima* lo pone sobre otra cosa y *caer* lo deja caer sobre lo de
     debajo;
   - *copiar* lo duplica; *quitar* o *borrar* lo sacan del mundo;
   - *deshacer* vuelve a su versión de antes;
   - *escribir* o *editar* meten sus palabras como código dentro;
   - *soltar* lo deja.

6. **Juegos y eventos.** Algunas palabras hacen que lo siguiente que pongan
   tenga vida:
   - *puerta*, *abrir*, *entrada*: una puerta que se abre y se cierra con
     **E**.
   - *saltar*, *trampolín*, *rebotar*: un trampolín; al pisarlo te lanza
     hacia arriba.
   - *portal*, *cruzar*, *viajar*: un portal que te lleva al sitio que
     *recordaron* o a donde acabó la frase. Si es el mismo sitio, te lleva a
     otro nivel, arriba o abajo.
   - *premio*, *tesoro*, *moneda*, *jugar*: un premio que gira. Al tocarlo
     da un punto y vuelve al minuto. Los seres que estén cerca van a por él.
   - *campana*, *tambor*, *nota*: suena al tocarlo.
   - *hola*, *saludar*, *historia*: la pieza dice la frase cuando llegas.
   - *agua*, *viento*, *bosque*, *fuego*, *magia*, *máquina*: además, la
     pieza suena a eso.
7. **Palabras que aprenden.** Al principio, una palabra suya (de Abla o
   inventada) solo es su forma. Le dan sentido de dos maneras:
   - diciéndolo: *kevo significa subir*;
   - Abla, con su propio diccionario: cuando uno de sus seres dice una frase
     con su traducción palabra por palabra, cada palabra suya que se traduce
     por algo que ya entienden pasa a significar eso. Por ejemplo, de «mesu
     mika» («cielo luz») aprende *mesu = cielo* y *mika = luz*.

   Desde entonces, decir esa palabra hace lo que significa: así su idioma se
   vuelve un idioma para construir. Lo que aprenden se guarda.

Cada pieza es código Go escrito así. Si les enseñas vídeos (`ven …`), lo que
ven entra en lo que dicen y piensan, y de ahí en lo que construyen.

```text
taller› palabras                  lo que han aprendido (y quién se lo dio)
taller› palabras zape premio      enseñarles tú: «zape» será un premio
taller› palabras olvida zape      que vuelva a ser solo su forma
taller› puntos                    el marcador de sus juegos
```

## Nexo: un cerebro nuevo con lo mejor de los tres

Nexo es una **tercera IA** del taller. No cambia a Nyx ni a Abla: es un
cerebro aparte, hecho con lo mejor de cada uno. Nace vacío, las escucha a las
dos (y a ti), ve lo mismo que ven y construye con ellas. Se turnan entre las
tres.

| Viene de | Qué hace en Nexo |
|---|---|
| **Nyx** | Las palabras son nudos unidos por lazos que se refuerzan al oírlas juntas y se aflojan si no se usan. Para contestar, la activación sale de lo que le dicen. Las palabras raras pesan más que las comunes y lo que llega por varios caminos se refuerza. El contexto empuja más flojo que la pregunta. Tiene un contexto vivo que se edita solo, y aprende más cuando algo le sorprende. |
| **Abla** | Sabe hechos con su confianza («agua es parte de río», «fuego causa luz»). Su lógica deduce («agua es parte de río» y «río es parte de mar» → «agua es parte de mar») y encuentra contradicciones. Cuando dos palabras van juntas muchas veces, inventa una palabra suya para las dos. Lleva un diario que no se borra. |
| **El cerebro web** | Sinestesia: lo que ve a la vez que oye una palabra se une a ella («bosque» acaba siendo verde) y construye con ese color. Cuando nadie le habla, sueña: repasa caminos de lo que sabe y los afianza. |

Y además mejora lo que a ninguna le salía bien:

- **Contesta con frases**: un camino por lo que sabe, un hecho y su color.
  Nyx contestaba con una sola palabra y Abla con plantillas.
- **Resuelve las contradicciones**: pierde la idea más débil.
- **La confianza crece con cada prueba** y depende de quién se lo diga.
- **Sus palabras inventadas construyen**: se las enseña al constructor.

```text
taller› nexo                  que hable ahora (y construya lo que dice)
taller› nexo ¿qué hay en el bosque?
taller› nexo estado           cuánto sabe
taller› nexo piensa           sus últimas deducciones, sueños y palabras nuevas
taller› nexo sabe agua        lo que sabe del agua, su color y con qué la une
taller› nexo palabras         las palabras que ha inventado
taller› nexo hereda           que lea lo que ya saben Nyx y Abla
```

`nexo hereda` solo **lee** los archivos de las otras, sin cambiar nada:

- los hechos de los 27 seres de Abla (su conocimiento en C);
- los lazos más fuertes de la cabeza de Nyx;
- los sitios que recuerda Nyx Mundo.

Si no lo pides, Nexo empieza sin nada, como ellas.

En el modo estudio, Nexo también ve los vídeos y perfecciona sus piezas.

## Modo estudio: una lista de vídeos para perfeccionar

```text
taller› estudia https://www.youtube.com/watch?v=… https://www.youtube.com/watch?v=…
taller› estudia ~/videos.txt          (un vídeo por línea)
taller› estudio                       cómo va (y lo que salió de cada vídeo)
taller› estudio para                  que paren al acabar el vídeo de ahora
```

Con cada vídeo de la lista:

1. **Lo ven las dos.** Nyx lo recuerda como sitio; Abla mira sus fotogramas.
   El taller espera a que Abla termine.
2. **Hablan de lo que vieron**, y lo que dicen se construye.
3. **Perfeccionan.** Cada una repasa sus piezas y mide cuánto se parecen al
   vídeo: sus colores, si es alto o ancho y cuánta luz tiene. Prueba cambios
   con sus manos:
   - teñir con los colores del vídeo;
   - poner de textura un fotograma;
   - un material parecido;
   - estirar;
   - dar luz.

   Solo se queda con los cambios que hacen que la pieza se parezca **más**. Si
   un cambio no mejora el parecido, lo deshace. En la charla lo ves: «Parecido
   35% → 77%».

Mientras estudian, los turnos de siempre esperan. La lista se guarda: si
cierras el taller, al volver siguen por donde iban.

## Tú también: crear con texto

```text
taller› crea tres piedra encima luz azul
taller› crea madera dos zela alrededor mika llamar fuente
taller› forma lista
taller› texturas
```

`crea …` usa las mismas reglas que ellas y empieza cerca de donde estás. En la
ventana puedes escribir «crea …» en la caja de la charla.

Tus propias imágenes sirven de textura: ponlas en
`~/.local/share/nyx-mundo/taller/texturas/` (png o jpg) y úsalas por su nombre
(*textura miimagen*).

## Godot

Si tienes Godot 4, escribe `godot` en el taller. El taller:

1. escribe un proyecto en `~/.local/share/nyx-mundo/taller/godot/`;
2. lo abre si encuentra Godot (si no, te dice qué carpeta importar).

Le das a ▶ (F5) y entras en su mundo, dibujado por Godot: luz y sombras,
materiales y texturas, y colisiones exactas con la física de Godot. Lo que se
mueve se mueve y lo que construyen aparece mientras lo construyen. Deja el
taller abierto: Godot le va pidiendo lo que hay.

Usa el modo «Compatibilidad» (OpenGL), así que va bien también con gráficos
Intel integrados. Controles: WASD, Espacio, V (volar), E (usar) y Esc (soltar
el ratón).

## Código y 3D

Cada pieza es un programa en Go: `~/.local/share/nyx-mundo/objetos/<pieza>.go`.

- Junto a cada pieza hay un modelo para Blender, `<pieza>.obj` + `<pieza>.mtl`,
  con sus colores. Lo que brilla va como material emisivo.
- Cuando alguien cambia una pieza, sale una versión nueva (`-v2`, `-v3`…) y
  la anterior se queda guardada.

| Orden | Qué hace |
|---|---|
| `exporta` | Todo el mundo en una escena: `taller/blender/mundo.obj`, con sus texturas en `.png` (sirve para Blender y para Godot). |
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
| `taller_lengua.go` | de palabras a construcción: el constructor de cada una, sus manos (el editor) y los eventos |
| `taller_vocabulario.go` | las palabras suyas a las que dan sentido |
| `taller_nexo.go` | Nexo, el cerebro nuevo hecho con lo mejor de los tres |
| `taller_materiales.go` | materiales y texturas (con nombre, de cualquier palabra, de fotos o tuyas) |
| `taller_formas_propias.go` | las formas que guardan con nombre |
| `taller_godot.go` | el proyecto de Godot que muestra su mundo en vivo |
| `taller_estudio.go` | el modo estudio: ver una lista de vídeos y perfeccionar lo hecho |
| `taller_motor.go` | el motor: partes, scripts, eventos, sonidos, física exacta, juegos |
| `taller_motor.js` | en la ventana: caminar con física, dibujar lo que se mueve, sonido 3D |
| `taller_abla.go` | el puente con Abla (el programa en C) |
| `instalar-abla.sh` | compila Abla, añade el taller a Nyx y recompila |

Lo que guardan, en `~/.local/share/nyx-mundo/`:

| Ruta | Qué hay |
|---|---|
| `taller/obra.json` | las piezas, quién las hizo y quién las cambió |
| `taller/charla.txt` | todo lo que se han dicho |
| `taller/palabras.json` | las palabras que aprendieron |
| `taller/nexo.json` | lo que sabe Nexo (lazos, hechos, colores, sus palabras) |
| `taller/nexo-diario.txt` | todo lo que ha pensado Nexo (no se borra) |
| `taller/videos/` | los vídeos que vieron |
| `abla/` | la memoria de Abla |
| `abla/fotos/vistas.jsonl` | lo que vio en cada fotograma |
