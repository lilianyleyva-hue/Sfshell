/* nyx.h — lo que comparten todos los archivos de Nyx (tipos y funciones).
 *
 * Nyx: el cerebro resonante de las 18 IAs, en C (C99).
 *   main.c      la interfaz (el programa empieza aquí)
 *   base.c      azar, textos y cómo es cada una de las 18
 *   resh.c      diccionario de la lengua Resh (datos en resh.h)
 *   texto.c     palabras y firmas
 *   mente.c     una mente: semiones, resonancia, veredicto, frases
 *   frases.c    memoria de frases (lo que leyeron, entero)
 *   consejo.c   las 18 juntas: imaginación, deliberación, opinión
 *   memoria.c   guardar y cargar
 */
#ifndef NYX_H
#define NYX_H

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <math.h>

#define NROLES 18
#define MAXSEM 1000        /* semiones por mente */
#define MAXVEC 16          /* acoples por semión */
#define DIMF 8             /* rasgos de la firma */
#define LARGO 28           /* bytes de una etiqueta (con el 0 final) */
#define HASHN 4096         /* tabla etiqueta -> semión (potencia de 2) */
#define NACC 6             /* cosas que una mente puede querer hacer */
#define NVEN 16            /* ventana de contexto (lo último que vivió) */
#define LVEN 112
#define MAXEST 24          /* palabras por estímulo */
#define NCAM 48            /* caminos A->B->C recordados */
#define MAXFRASE 24        /* palabras de una frase dicha */
#define MAXFR 1500         /* frases recordadas */
#define LFRASE 160
#define NYX_PI 3.14159265f
#define NYX_2PI 6.28318531f

typedef struct { const char *es; const char *resh; } ParResh;

enum { ACC_HABLAR, ACC_PREGUNTAR, ACC_IMAGINAR, ACC_SONAR, ACC_RECORDAR, ACC_ESCUCHAR };
typedef struct {
    const char *nombre;
    float ruido, rigidez, A0;   /* η, λ, A₀ de la dinámica */
    float sabeResh;             /* qué parte del Resh sabe al nacer */
    const char *objetivo;       /* lo que persigue */
    float gusto[NACC];          /* gusto innato por cada acción */
    int color;                  /* color ANSI 256 para la UI */
} RolInfo;
#define NYX_NCORPUS nyx_ncorpus
typedef struct {
    short j;
    float w;      /* asociación (resonancia) */
    float th;     /* desfase preferido: 0 acuerdo, π oposición */
    float sec;    /* secuencia: cuánto "este va seguido de j" */
} Acople;
typedef struct {
    char et[LARGO];
    float f[DIMF];
    float A, fase, frec;
    signed char carga;
    unsigned char fusion;   /* idea propia (a⊕b) */
    unsigned char sabe;     /* sabe decirla en Resh */
    unsigned char nv;
    unsigned short usos, dialogo;
    Acople v[MAXVEC];
} Semion;
typedef struct { short a, b, c; unsigned char k; } Camino;

typedef struct {
    int rol;
    int n;
    Semion s[MAXSEM];
    short hash[HASHN];
    float ruido, rigidez, A0;
    short obj[8]; int nobj;
    short foco[7]; int nfoco;          /* memoria de trabajo (7±2) */
    short epis[24]; int nepis;         /* memoria episódica */
    Camino cam[NCAM]; int ncam;
    int aciertos, intentos;
    float valor[NACC];                 /* lo aprendido: cuánto le conviene cada acción */
    int veces[NACC];
    int ultima, racha;
    short recientes[6]; int nrec;      /* lo que ganó hace poco (fatiga) */
    char ven[NVEN][LVEN]; int iven, nven;
    long ciclos, dichos, ideas, cristales, reshAprendidas;
} Mente;
enum { EV_PIENSA, EV_DICE, EV_APRENDE, EV_IDEA, EV_RESH, EV_SUENA, EV_NOTA };
typedef void (*NyxEvento)(void *ud, int rol, int tipo, const char *texto);
typedef struct {
    int rol;
    char atractor[LARGO];
    float C, E, peso;
    int acuerdo;
} NyxVoto;
typedef struct {
    char frase[420];
    char resh[420];
    char ganador[LARGO];
    int vocero;
    float acuerdo;
    NyxVoto votos[NROLES];
    int nvotos;
    int recordada;          /* 1: la recordó de lo que leyó · 0: la armó ella */
} NyxRespuesta;

/* Una frase que leyeron, entera (frases.c). */
typedef struct {
    char t[LFRASE];
    float puntos;           /* tu opinión: bien suma, mal resta */
    unsigned short usos;
} FraseMem;
typedef struct {
    Mente m[NROLES];
    unsigned long tick;
    int ultimo;
    NyxEvento ev;
    void *ud;
    /* para 'bien' y 'mal': qué se dijo y quién lo pensó */
    int fbValido, fbN;
    int fbMente[NROLES], fbGan[NROLES], fbNest[NROLES];
    int fbEst[NROLES][MAXEST];
    int fbFrase[MAXFRASE], fbNfrase, fbVocero;
    char ecos[10][200]; int iecos;     /* lo último que se dijo (anti-eco) */
    int fbFr, fbRecordada;             /* la frase recordada que se dijo (o -1) */
    FraseMem fr[MAXFR]; int nfr;       /* memoria de frases */
} Consejo;

/* datos compartidos */
extern const char *NYX_ACC[NACC];
extern const RolInfo NYX_ROLES[NROLES];
extern const char *NYX_CORPUS[];
extern const int nyx_ncorpus;

/* funciones */
void nyx_semilla(unsigned long long s);
unsigned nyx_u32(void);
unsigned nyx_fnv(const char *s);
void nyx_copia(char *dst, const char *src, size_t cap);
int nyx_rol_de(const char *nombre);
int nyx_vacia(const char *w);
int nyx_conector(const char *w);
void nyx_dic_init(void);
const char *nyx_a_resh(const char *es);
const char *nyx_a_es(const char *resh);
int nyx_es_particula(const char *es);
int nyx_tokens(const char *t, char toks[][LARGO], int max);
void nyx_firma(const char *w, float *f);
float m_precision(const Mente *m);
void m_ventana(Mente *m, const char *texto);
int m_busca(const Mente *m, const char *et);
void m_hash_pon(Mente *m, int i);
void m_hash_rehace(Mente *m);
int m_nuevo(Mente *m, const char *et, int fusion);
Acople *m_acople(Mente *m, int a, int b);
Acople *m_acople_nuevo(Mente *m, int a, int b, float w, float th);
void m_fija(Mente *m, int a, int b, float w, float th);
void m_refuerza(Mente *m, int a, int b, float dw, float th, float dsec);
float m_coh(const Mente *m, int i);
void m_envuelve(float *f);
void m_decae(Mente *m, int i, float ruido);
void m_propaga(Mente *m, int i);
int m_region(const Mente *m, const int *est, int ne, int *out, int tope);
void m_atiende(Mente *m, int id);
void m_episodio(Mente *m, int id);
int m_es_objetivo(const Mente *m, int id);
void m_poda(Mente *m);
int m_ingesta2(Mente *m, const char *texto, int *ids, int max, int dialogo, float gramatica);
int m_ingesta(Mente *m, const char *texto, int *ids, int max, int dialogo);
int m_ingesta2(Mente *m, const char *texto, int *ids, int max, int dialogo, float gramatica);
int m_predice(const Mente *m, int *out, int max);
int m_recibe(Mente *m, const char *texto, const char *quien, int *ids, int max);
int m_salto(const Mente *m, int id, const int *vis, int nvis);
int m_cristaliza(Mente *m, int a, int b, int c);
int m_transitiva(Mente *m, int id);
float m_incrustacion(const Mente *m, int id);
int m_veredicto(Mente *m, const int *est0, int ne0, float *C, float *E, int *cristal);
float m_en_contexto(const int *ctx, int nctx, int id);
int m_frase(Mente *m, int centro, int *out, int max, const int *ctx, int nctx);
void m_texto(const Mente *m, const int *ids, int n, char *es, size_t ces, char *resh, size_t cre);
int m_imagina(Mente *m, int *pa, int *pb);
void m_cansa(Mente *m, const int *ids, int n);
void m_piensa(Mente *m);
void m_init(Mente *m, int rol);
int c_es_eco(const Consejo *c, const char *es);
void c_pon_eco(Consejo *c, const char *es);
void c_evento(Consejo *c, int rol, int tipo, const char *texto);
void consejo_init(Consejo *c, unsigned long long semilla, int conInfancia);
int consejo_lee(Consejo *c, const char *texto);
float c_difunde(Consejo *c, int emisor, const int *frase, int nf, int *aprendieron, char *palabraAprendida);
int c_ya_dicho(const Mente *m, const char *es);
int c_tema(const Mente *m);
int c_duda(const Mente *m);
void c_aprende(Consejo *c, int rol, int acc, float recompensa);
float c_puntaje(const Mente *m, int a);
float c_habla(Consejo *c, int rol, int centro, const char *prefijo);
void consejo_paso(Consejo *c);
void consejo_delibera(Consejo *c, const char *pregunta, NyxRespuesta *out);
void consejo_habla_con(Consejo *c, int rol, const char *texto, char *es, size_t ces, char *resh, size_t cre, float *C);
int consejo_opina(Consejo *c, int bueno);
int consejo_guarda(const Consejo *c, const char *ruta);
int consejo_carga2(Consejo *c, Consejo *tmp0, FILE *f);
int consejo_carga(Consejo *c, const char *ruta);
int consejo_carga2(Consejo *c, Consejo *tmp0, FILE *f);
int m_cuenta_resh(const Mente *m);
int m_mejor_accion(const Mente *m);


/* frases.c */
int frases_pon(Consejo *c, const char *texto);
int frases_elige(const Consejo *c, const Mente *m, int centro, const int *ctx, int nctx, float *puntaje);
void frases_resh(const Mente *m, const char *es, char *resh, size_t cap);
void frases_opina(Consejo *c, int k, int bueno);

/* ayudas pequeñas */
static inline float nyx_f01(void) { return (float)(nyx_u32() >> 8) * (1.0f / 16777216.0f); }
static inline float nyx_rango(float a, float b) { return a + (b - a) * nyx_f01(); }
static inline int nyx_ent(int n) { return n <= 0 ? 0 : (int)(nyx_u32() % (unsigned)n); }
static inline float nyx_min(float a, float b) { return a < b ? a : b; }
static inline float nyx_max(float a, float b) { return a > b ? a : b; }
static inline float nyx_lim(float x, float a, float b) { return x < a ? a : (x > b ? b : x); }

#endif
