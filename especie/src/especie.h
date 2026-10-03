#ifndef ABLA_ESPECIE_H
#define ABLA_ESPECIE_H
/* Estado interno de la especie, compartido por especie.c (cómo piensan)
 * y comandos.c (cómo se les habla desde la terminal o la interfaz). */
#include "comun.h"

enum { LENGUAJE = 0, DATOS = 1, LOGICA = 2 };

typedef struct {
  int de;          /* DE_HUMANO o el id del ser */
  int cpp;         /* 1: data C exacta (un struct hecho); 0: frase en Abla */
  int n;           /* palabras de la frase */
  int palabras[6];
  struct hecho dato;
} Mensaje;

#define BUZON 256
#define HISTORIAL 200
#define COOC 2048
#define CORRIENTE 1000
#define RED 512

typedef struct {
  int id;
  char nombre[16];
  int tribu;
  int concepto; /* la palabra de la que nace su nombre */
  Memoria memoria;
  Saber saber;
  uint8_t conoce[MAX_PALABRAS / 8]; /* palabras compuestas que conoce */
  Shell shell;
  Mensaje buzon[BUZON];
  int bini, bn;
  uint32_t cooc_clave[COOC];
  int cooc_valor[COOC];
  char* historial[HISTORIAL];
  int hini, hn;
  uint64_t pensamientos, enviados, recibidos;
  char* ultimo;
  char* respuesta; /* lo que respondió al último mensaje del humano */
} Ser;

typedef struct {
  uint64_t n;
  int ser;
  char* texto;
} Linea;

typedef struct {
  char mundo[RUTA_MAX];
  Ser seres[NUM_SERES];
  Shell yo; /* la terminal del humano */
  uint64_t tick, mensajes_abla, mensajes_c;
  Linea corriente[CORRIENTE];
  uint64_t nlinea;
  struct {
    int de, para, c;
  } red[RED];
  uint64_t nred;
  int ritmo;
  uint64_t rng;
  int iniciada;
  double ultimo_ms;
} Especie;

extern Especie E;

const char* nombre_tribu(int t);
const char* nombre_de(int id);
const char* glosa_hecho(const struct hecho* h);
Ser* buscar_ser(const char* q);
void entregar(int para, const Mensaje* m);
void pensar(Ser* s);
void anotar(Ser* s, const char* texto, int publico);
int frase_de(const struct hecho* h, const char* marca, int* out);
int conoce_palabra(const Ser* s, int palabra);
double ahora_ms(void);
int tui_ejecutar(int color); /* 0 si no hay terminal interactiva */

#endif
