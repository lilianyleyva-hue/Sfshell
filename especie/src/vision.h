#ifndef ABLA_VISION_H
#define ABLA_VISION_H
/* Visión: la especie mira fotos (en colores reales) y aprende de ellas. */
#include "especie.h"

enum { V_ABRIR, V_MIRAR, V_COLORES, V_BORDES, V_REGIONES, V_DESCRIBIR, V_COMPARTIR, V_PASOS };

typedef struct {
  char nombre[16];
  uint8_t r, g, b; /* el color medio real de esa zona del espectro */
  float frac;
} Tono;

typedef struct {
  int activa;
  char nombre[256];
  int w, h;
  uint8_t* rgb; /* la foto tal como llegó (hasta 320×240) */
  int aw, ah;
  uint8_t* vista; /* lo que ven: la foto a su agudeza de visión */
  int paso, ser_paso;
  Tono paleta[6];
  int npaleta;
  float brillo, contraste, bordes, bordes_h, bordes_v;
  char arriba[16], abajo[16];
  int conceptos[6];
  float pesos[6];
  int nconceptos;
  char descripcion[600];
  char abla[200];
  /* estadísticas */
  int en_cola;
  unsigned long vistas;
  double ms_por_foto, inicio_ms;
} Vision;

extern Vision V;

const char* paso_vision_nombre(int paso);
const char* vision_nombre_bonito(const char* n);
void vision_iniciar(void);
void vision_paso(void);
void vision_describir(Texto* t);
int ppm_leer(const char* ruta, int* w, int* h, uint8_t** rgb);
int ppm_escribir(const char* ruta, int w, int h, const uint8_t* rgb);

#endif
