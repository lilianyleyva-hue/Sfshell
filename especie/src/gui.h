#ifndef ABLA_GUI_H
#define ABLA_GUI_H
/* Interfaz gráfica dibujada en C, píxel a píxel, en una imagen RGBA.
 * Quien la muestre (la página en Code App, o en el futuro una ventana nativa)
 * solo copia esa imagen a la pantalla y le pasa los toques y las teclas. */
#include <stdint.h>

/* ---------- letras (src/fuente.c, generado por herramientas/fuente.py) ---------- */
typedef struct {
  uint32_t cp;
  int16_t w, h, xoff, yoff, avance;
  uint32_t off; /* en fuente_datos: 4 bits por píxel */
} Glifo;

typedef struct {
  int px, negrita, asc, desc, n;
  const Glifo* g;
} Fuente;

extern const uint8_t fuente_datos[];
extern const Fuente fuentes[];
extern const int nfuentes;

/* ---------- lienzo ---------- */
typedef struct {
  uint32_t* px; /* RGBA en memoria (byte R primero) */
  int w, h;
  int cx0, cy0, cx1, cy1; /* recorte */
} Lienzo;

extern Lienzo L;

void lz_tamano(int w, int h);
void lz_recorte(int x, int y, int w, int h);
void lz_sin_recorte(void);
void lz_rect(int x, int y, int w, int h, uint32_t rgb, int a);
void lz_rrect(float x, float y, float w, float h, float r, uint32_t rgb, int a);
void lz_rrect_borde(float x, float y, float w, float h, float r, float grosor, uint32_t rgb, int a);
void lz_circulo(float cx, float cy, float r, uint32_t rgb, int a);
void lz_linea(float x0, float y0, float x1, float y1, float grosor, uint32_t rgb, int a);
void lz_imagen(const uint8_t* rgb, int iw, int ih, int x, int y, int w, int h, int suave);

const Fuente* fuente(int px, int negrita);
int lz_ancho(const Fuente* f, const char* s);
int lz_texto(const Fuente* f, int x, int y, const char* s, uint32_t rgb, int a);
int lz_texto_max(const Fuente* f, int x, int y, int max, const char* s, uint32_t rgb, int a); /* recorta con … */
int lz_parrafo(const Fuente* f, int x, int y, int ancho, const char* s, uint32_t rgb, int a, int max_lineas,
               int dibujar); /* devuelve el alto */
int lz_alto_linea(const Fuente* f);

#endif
