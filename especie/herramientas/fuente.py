#!/usr/bin/env python3
"""Convierte la letra DejaVu Sans en tablas de C (src/fuente.c) para que la
interfaz dibuje el texto ella misma, sin depender del sistema.

Cada letra se guarda recortada, con 16 niveles de transparencia (4 bits por
píxel) para que los bordes se vean suaves. Uso: python3 herramientas/fuente.py
DejaVu: licencia libre (Bitstream Vera + dominio público), ver dejavu-fonts.github.io."""
import sys
from PIL import Image, ImageDraw, ImageFont

REGULAR = "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"
NEGRITA = "/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf"
CARAS = [("normal", REGULAR, 22), ("normal", REGULAR, 26), ("normal", REGULAR, 30),
         ("negrita", NEGRITA, 26), ("negrita", NEGRITA, 30), ("negrita", NEGRITA, 36),
         ("negrita", NEGRITA, 44), ("negrita", NEGRITA, 64)]
LETRAS = [chr(c) for c in range(32, 127)] + list("áéíóúüñÁÉÍÓÚÜÑ¿¡«»·×÷°²³ºª€—–…•●◉▪→←↑↓✓✗★−≈≤≥√∞‹›⏪⏩")

datos = bytearray()
salida = ["/* Generado por herramientas/fuente.py a partir de DejaVu Sans. No editar.",
          " * DejaVu Sans: licencia libre (Bitstream Vera + dominio público). */",
          '#include "gui.h"', ""]
caras_c = []
for i, (estilo, ruta, px) in enumerate(CARAS):
    f = ImageFont.truetype(ruta, px)
    asc, desc = f.getmetrics()
    glifos = []
    for ch in sorted(set(LETRAS), key=ord):
        try:
            caja = f.getbbox(ch)
        except Exception:
            continue
        if not f.getmask(ch).getbbox() and ch != " ":
            continue  # la letra no tiene este carácter
        avance = round(f.getlength(ch))
        w, h = max(0, caja[2] - caja[0]), max(0, caja[3] - caja[1])
        off = len(datos)
        if w and h:
            im = Image.new("L", (w, h), 0)
            ImageDraw.Draw(im).text((-caja[0], -caja[1]), ch, font=f, fill=255)
            px4 = [v >> 4 for v in im.tobytes()]
            if len(px4) % 2:
                px4.append(0)
            for k in range(0, len(px4), 2):
                datos.append(px4[k] | (px4[k + 1] << 4))
        glifos.append((ord(ch), w, h, caja[0], caja[1], avance, off))
    salida.append(f"static const Glifo glifos{i}[] = {{")
    for g in glifos:
        salida.append("  {%d, %d, %d, %d, %d, %d, %d}," % g)
    salida.append("};")
    caras_c.append(f"  {{{px}, {1 if estilo == 'negrita' else 0}, {asc}, {desc}, {len(glifos)}, glifos{i}}},")

salida.append("const uint8_t fuente_datos[] = {")
for k in range(0, len(datos), 24):
    salida.append("  " + ",".join(str(b) for b in datos[k:k + 24]) + ",")
salida.append("};")
salida.append("const Fuente fuentes[] = {")
salida += caras_c
salida.append("};")
salida.append(f"const int nfuentes = {len(CARAS)};")
open(sys.argv[1] if len(sys.argv) > 1 else "src/fuente.c", "w").write("\n".join(salida) + "\n")
print(f"{len(CARAS)} tamaños, {len(datos)} bytes de letras")
