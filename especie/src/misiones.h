#ifndef ABLA_MISIONES_H
#define ABLA_MISIONES_H
/* Misiones de matemáticas: la especie resuelve problemas en equipo y sube de nivel. */
#include "especie.h"

enum { OP_SUMA, OP_RESTA, OP_MULT, OP_DIV, OP_POT, OP_ECUACION, OP_PRIMO, OP_SUCESION, NOPS };
enum { FASE_LEER, FASE_CALCULAR, FASE_VERIFICAR, FASE_DECIDIR, FASE_RESULTADO, NFASES };

#define MAX_LOGROS 32
#define MAX_HIST 12

typedef struct {
  int nivel, aciertos, meta;
  unsigned long resueltos, fallados;
  /* problema actual */
  int op, del_humano;
  long respuesta;
  char enunciado[128];
  int fase, reintentos;
  int lector, calc[3], verif[2];
  long propuesta[3];
  int aprobada[3][2]; /* -1 sin mirar, 0 rechazada, 1 aprobada */
  long decision;
  char paso[256];      /* qué está pasando ahora, en una frase */
  struct {
    char texto[160];
    int bien;
  } hist[MAX_HIST];
  int nhist;
  float habilidad[NUM_SERES][NOPS];
  char logros[MAX_LOGROS][128];
  int nlogros;
  uint64_t celebrar_hasta; /* tick hasta el que se celebra la subida de nivel */
  long humano_a, humano_b;
  int humano_op, humano_pendiente;
} Misiones;

extern Misiones M;

const char* op_nombre(int op);
const char* fase_nombre(int fase);
const char* nivel_nombre(int nivel);
void agudeza_de_nivel(int nivel, int* w, int* h);
void misiones_iniciar(void);
void misiones_paso(void);
void misiones_guardar(void);
void misiones_describir(Texto* t);
int misiones_problema_humano(const char* expr, Texto* out);

#endif
