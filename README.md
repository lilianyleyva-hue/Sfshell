# Nuevo Cerebro IA

Este repositorio tiene dos partes:

- **Interfaz web** (esta página): un cerebro que ve y escucha de forma diferente.
- **[Especie Abla](especie/README.md)**: 27 mentes con el cerebro en C que hablan su propio idioma de 4000 palabras, guardan su conocimiento como código C, tienen una interfaz visual ([`especie/web/index.html`](especie/web/index.html)) y una terminal estilo bash, y no dejan de pensar.

Una interfaz para un cerebro de IA que **no ve ni escucha como nosotros**.

- **Ver** 👁 — no reconoce objetos. Percibe el mundo como un campo de *movimiento, luz y color* (una rejilla de 16×12). La vista previa nunca muestra la imagen de la cámara, solo lo que el cerebro siente de ella.
- **Escuchar** 👂 — no empieza por las palabras. Primero siente *ritmo, tono e intensidad* (24 bandas de frecuencia, como el oído). Si el dictado del dispositivo está disponible, las palabras llegan después.
- **Soñar** ✦ — sin cámara ni micrófono: el cerebro imagina formas y sonidos.
- **Sinestesia** — cuando algo se mueve y suena a la vez, el cerebro une ese color con ese tono. Cuantas más veces ocurre, más fuerte queda la asociación (aprendizaje de Hebb en la red).

Las neuronas azules son la corteza visual, las rosas la auditiva y las verdes la zona de asociación, donde se mezclan los sentidos.

## Usarlo en iPad (iPadOS 18.6)

Es una web estática (HTML + CSS + JS, sin dependencias). Safari solo da acceso a la cámara y al micrófono por **HTTPS**, así que:

1. Publica la carpeta con HTTPS, por ejemplo con GitHub Pages (Settings → Pages → rama de este proyecto).
2. Abre la URL en Safari en el iPad y toca **Ver** o **Escuchar**; acepta los permisos.
3. Opcional: Compartir → **Añadir a pantalla de inicio** para usarlo a pantalla completa como una app.

Para que aparezcan palabras al escuchar, el **Dictado** debe estar activado (Ajustes → General → Teclado → Activar Dictado). Sin él, el cerebro sigue escuchando tono y ritmo.

## Probarlo en un ordenador

```sh
python3 -m http.server 8000
# abre http://localhost:8000
```

## Taller de Nyx y Abla

En [`nyx-abla/`](nyx-abla/LEEME.md), Abla trabaja junto a Nyx (Debian) sin cambiar el cerebro de Nyx. Las dos ven vídeos de YouTube, escriben código Go que construye objetos 3D (exportables a Blender) y crean juntas un mundo con sus recuerdos. Cada una puede cambiar lo que hizo la otra.
