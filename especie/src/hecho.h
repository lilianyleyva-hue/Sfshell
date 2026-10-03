#ifndef ABLA_HECHO_H
#define ABLA_HECHO_H
#include <stdint.h>

/* Un hecho: sujeto -relacion-> objeto.
 * Sujeto y objeto son ids de palabras de Abla (ver ../abla/diccionario.txt).
 * Relaciones: 0 es, 1 parte de, 2 causa, 3 igual a, 4 opuesto a.
 * fuente: 0-26 = otro ser, 100 = innato, 200 = humano. */
struct hecho {
  uint16_t s;
  uint8_t r;
  uint16_t o;
  float confianza;
  uint32_t fuente;
};

#endif
