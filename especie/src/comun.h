#ifndef ABLA_COMUN_H
#define ABLA_COMUN_H
/* Especie Abla — declaraciones compartidas por todos los módulos en C. */

/* Funciones POSIX (clock_gettime, strtok_r, mkdir…) también con -std=c11 estricto. */
#ifndef _POSIX_C_SOURCE
#define _POSIX_C_SOURCE 200809L
#endif

#include <ctype.h>
#include <stdarg.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "hecho.h"

#define RAICES 200
#define ASPECTOS 20
#define BASE (RAICES * ASPECTOS) /* 4000 palabras innatas */
#define MAX_PALABRAS 16384
#define NUM_SERES 27
#define FORMA_MAX 24
#define RUTA_MAX 512

enum { ES = 0, PARTE = 1, CAUSA = 2, IGUAL = 3, OPUESTO = 4, NREL = 5 };
enum { ASP_BASE = 0, ASP_MUCHOS = 1, ASP_NEGADO = 2, ASP_PREGUNTA = 3, ASP_QUIEN = 16 };
enum { FUENTE_SEMILLA = 100, FUENTE_HUMANO = 200 };
#define DE_HUMANO (-1)

/* En WebAssembly, las funciones públicas se exportan para la interfaz web. */
#if defined(__wasm__)
#define EXPORTA(n) __attribute__((export_name(n)))
#else
#define EXPORTA(n)
#endif

/* ---------- util.c: memoria y texto que crece ---------- */
typedef struct {
  char* p;
  size_t n, cap;
} Texto;

void* xmalloc(size_t n);
void* xcalloc(size_t n, size_t t);
void* xrealloc(void* p, size_t n);
char* xstrdup(const char* s);
void tx_iniciar(Texto* t);
void tx_liberar(Texto* t);
void tx_vaciar(Texto* t);
void tx_addn(Texto* t, const char* s, size_t n);
void tx_add(Texto* t, const char* s);
void tx_printf(Texto* t, const char* fmt, ...);
void tx_json(Texto* t, const char* s); /* añade s como cadena JSON con comillas */
void minusculas(char* s);

/* ---------- idioma.c: Abla ---------- */
const char* idioma_raiz(int r);
const char* idioma_aspecto(int a);
const char* rel_nombre(int r);
const char* rel_glosa(int r);
int rel_de_nombre(const char* s);

void idioma_iniciar(void);
int idioma_tamano(void);
int idioma_compuestas(void);
int idioma_valida(int id);
int idioma_id(int raiz, int aspecto);
int idioma_raiz_de(int id);
int idioma_es_base(int id);
const char* idioma_forma(int id);
void idioma_glosa(int id, Texto* out);
const char* idioma_glosa_tmp(int id); /* búfer rotativo: válido hasta 8 llamadas después */
int idioma_de_forma(const char* f);
int idioma_de_espanol(const char* w);
int idioma_relacion(int rel);
int idioma_rel_de_palabra(int palabra);
int idioma_acunar(int a, int b);
int idioma_compuesta_de(int a, int b);
void idioma_partes(int id, int* a, int* b);
int idioma_leer(const char* texto, int* ids, int max, Texto* desconocidas);
void idioma_frase(const int* ids, int n, Texto* out);
void idioma_glosa_frase(const int* ids, int n, Texto* out);
void idioma_exportar(const char* ruta);
void idioma_guardar_compuestas(const char* ruta);
void idioma_cargar_compuestas(const char* ruta);

/* ---------- archivos.c: carpetas y archivos (POSIX + stdio) ---------- */
typedef struct {
  char nombre[256];
  int directorio;
  long long tamano;
} Entrada;

void ruta_unir(char* out, size_t n, const char* a, const char* b);
const char* ruta_nombre(const char* p);
int arch_existe(const char* p);
int arch_es_directorio(const char* p);
int arch_es_archivo(const char* p);
long long arch_tamano(const char* p);
int arch_crear_directorios(const char* p);
int arch_listar(const char* p, Entrada** out); /* devuelve cuántas; liberar con free */
int arch_borrar_todo(const char* p);
int arch_renombrar(const char* a, const char* b);
int arch_copiar(const char* a, const char* b);
int arch_leer(const char* p, Texto* out);
int arch_escribir(const char* p, const char* datos, size_t n, int anexar);

/* ---------- memoria.c: contexto + largo plazo ---------- */
typedef struct {
  uint64_t tick;
  char* texto;
  uint32_t tokens;
} Recuerdo;

typedef struct {
  Recuerdo* r;
  size_t cap, ini, n;
  uint64_t usados, capacidad, total;
  char ruta[RUTA_MAX];
  FILE* log;
  long max_bytes; /* 0 = sin límite: nunca olvida */
} Memoria;

void memoria_iniciar(Memoria* m, uint64_t capacidad, const char* ruta, long max_bytes);
void memoria_recordar(Memoria* m, uint64_t tick, const char* texto);
const Recuerdo* memoria_en(const Memoria* m, size_t i);
void memoria_buscar(Memoria* m, const char* q, size_t max, Texto* out);
long long memoria_bytes(const Memoria* m);
void memoria_sincronizar(Memoria* m);

/* ---------- saber.c: hechos, guardados como código C ---------- */
typedef struct {
  struct hecho* h;
  int32_t* siguiente;
  size_t n, cap;
  uint64_t* claves;
  int32_t* valores;
  size_t tcap;
  int32_t* primero; /* por sujeto */
} Saber;

#define MAX_HECHOS 50000
void saber_iniciar(Saber* s);
int saber_agregar(Saber* s, struct hecho h);
struct hecho* saber_buscar(Saber* s, int su, int r, int o);
int saber_desde(const Saber* s, int su, int r, int* out, int max);
int saber_exportar_c(const Saber* s, const char* ruta, const char* nombre, const char* encabezado);
int saber_importar_c(Saber* s, const char* ruta);
extern const char* HECHO_H;

/* ---------- shell.c: terminal estilo bash ---------- */
typedef struct {
  char raiz[RUTA_MAX];
  char home[RUTA_MAX];
  char cwd[RUTA_MAX];
} Shell;

void shell_iniciar(Shell* sh, const char* raiz, const char* home);
int shell_resolver(const Shell* sh, const char* arg, char* out, size_t n);
void shell_real(const Shell* sh, const char* virt, char* out, size_t n);
const char* shell_mostrar(const Shell* sh, const char* v);
int shell_ejecutar(Shell* sh, int argc, char** argv, Texto* out); /* 0 si no es comando de terminal */
int trocear(const char* linea, char*** argv);
void liberar_args(int argc, char** argv);

/* ---------- especie.c y comandos.c: la API pública ---------- */
int abla_iniciar(const char* mundo, unsigned contexto, long max_mem_bytes);
void abla_ciclo(void);
int abla_ciclos_atrasados(int maximo);
const char* abla_ejecutar(const char* linea);
const char* abla_estado_json(void);
const char* abla_corriente_json(double desde);
const char* abla_prompt(void);
void abla_guardar(void);
void abla_fijar_hablante(const char* nombre);
int abla_ritmo(void);
void abla_fijar_ritmo(int ms);
uint64_t abla_tick(void);

#endif
