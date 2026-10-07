#!/bin/sh
# Pone a Abla junto a Nyx (Debian 13, sin sudo salvo para instalar paquetes).
#
#   sh instalar-abla.sh /ruta/a/nyx-go
#
# 1. Compila Abla (el programa en C de ../especie) y lo deja en
#    ~/.local/share/nyx-mundo/abla/abla
# 2. Copia los archivos del taller (taller_*.go) en la carpeta de Nyx.
#    No cambia NINGÚN archivo de Nyx: solo añade estos.
# 3. Recompila Nyx con su propio instalar.sh.
set -e
aqui="$(cd "$(dirname "$0")" && pwd)"
nyx="${1:-}"
if [ -z "$nyx" ]; then
    for c in "$HOME/nyx-go" "$HOME/nyx" "$HOME/Descargas/nyx-go" "$HOME/Downloads/nyx-go" "$aqui/../nyx-go"; do
        if [ -f "$c/mente.go" ] && [ -f "$c/instalar.sh" ]; then nyx="$c"; break; fi
    done
fi
if [ -z "$nyx" ] || [ ! -f "$nyx/mente.go" ]; then
    echo "¿Dónde está Nyx? Dímelo:  sh instalar-abla.sh /ruta/a/nyx-go"
    exit 1
fi
nyx="$(cd "$nyx" && pwd)"

falta=""
command -v cc   >/dev/null 2>&1 || falta="$falta build-essential"
command -v make >/dev/null 2>&1 || falta="$falta make"
command -v go   >/dev/null 2>&1 || falta="$falta golang-go"
if [ -n "$falta" ]; then
    echo "Falta:$falta"
    echo "  sudo apt install$falta"
    exit 1
fi

echo "1/3 Compilando a Abla (C)…"
make -C "$aqui/../especie" abla
mkdir -p "$HOME/.local/share/nyx-mundo/abla"
install -m 755 "$aqui/../especie/abla" "$HOME/.local/share/nyx-mundo/abla/abla"

echo "2/3 Añadiendo el taller a Nyx ($nyx)…"
for f in "$aqui"/taller_*.go "$aqui"/taller_*.js; do
    install -m 644 "$f" "$nyx/"
done

echo "3/3 Compilando Nyx…"
(cd "$nyx" && sh ./instalar.sh)
ln -sf "$HOME/.local/bin/nyx" "$HOME/.local/bin/nyx-taller"

echo
echo "Listo. Para abrir el taller:   nyx taller      (o  nyx-taller)"
echo "Y dentro:  camina   ·   ven https://www.youtube.com/watch?v=…   ·   ayuda"
falta=""
command -v ffmpeg >/dev/null 2>&1 || falta="$falta ffmpeg"
command -v yt-dlp >/dev/null 2>&1 || falta="$falta yt-dlp"
if [ -n "$falta" ]; then
    echo "Para que vean vídeos:  sudo apt install$falta"
fi
