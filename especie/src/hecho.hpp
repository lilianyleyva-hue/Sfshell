#pragma once
#include <cstdint>
// Un hecho: sujeto -relacion-> objeto. Sujeto y objeto son ids de palabras de Abla.
// Relaciones: 0 es, 1 parte de, 2 causa, 3 igual a, 4 opuesto a.
// fuente: 0-26 = otro ser, 100 = innato, 200 = humano.
struct Hecho {
  std::uint16_t s;
  std::uint8_t r;
  std::uint16_t o;
  float confianza;
  std::uint32_t fuente;
};
