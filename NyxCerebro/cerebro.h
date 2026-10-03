/* cerebro.h — Cerebro resonante de las 18 IAs de Nyx, en C (C99).
 *
 * Es el mismo paradigma del CerebroResonante de Nyxshell (Swift), llevado a C
 * y mejorado:
 *   - Cada mente es un fluido de "semiones" (palabras/ideas) con amplitud A,
 *     fase y carga. Pensar = dejar que las fases se sincronicen (Kuramoto)
 *     hasta que gana un atractor de coherencia.
 *   - MEJORA: memoria de secuencia. Además de la asociación, cada acople
 *     guarda cuánto "A va seguido de B". Así las mentes responden con frases
 *     en español aprendidas de lo que leen y oyen, no con palabras sueltas.
 *   - MEJORA: las palabras vacías (el, la, de, que…) no ganan veredictos.
 *   - MEJORA: la lengua Resh se reparte: cada mente sabe una parte, pregunta
 *     lo que no sabe ("ye árbol?") y las demás le enseñan.
 *   - MEJORA: imaginación con aprendizaje por refuerzo. Cada mente imagina
 *     qué quiere hacer (hablar, preguntar, imaginar, soñar, recordar,
 *     escuchar), lo hace, mide cómo le fue y aprende qué le conviene.
 *   - MEJORA: tu opinión (bien/mal) reestructura lo aprendido: lo malo queda
 *     en oposición de fase y pierde futuros veredictos.
 *   - Persistencia en un archivo de texto.
 *
 * Solo usa la biblioteca estándar de C: compila con clang/gcc y en Code App.
 */
#ifndef NYX_CEREBRO_H
#define NYX_CEREBRO_H

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <math.h>
#include "resh.h"

#define NROLES 18
#define MAXSEM 1400        /* semiones por mente */
#define MAXVEC 16          /* acoples por semión */
#define DIMF 8             /* rasgos de la firma */
#define LARGO 28           /* bytes de una etiqueta (con el 0 final) */
#define HASHN 4096         /* tabla etiqueta -> semión (potencia de 2) */
#define NACC 6             /* cosas que una mente puede querer hacer */
#define NVEN 16            /* ventana de contexto (lo último que vivió) */
#define LVEN 112
#define MAXEST 24          /* palabras por estímulo */
#define NCAM 48            /* caminos A->B->C recordados */
#define MAXFRASE 12
#define NYX_PI 3.14159265f
#define NYX_2PI 6.28318531f

/* ------------------------------------------------------------------ */
/*  Azar (xorshift: igual en todas las plataformas)                    */
/* ------------------------------------------------------------------ */

static unsigned long long nyx_azar_estado = 88172645463325252ULL;

static void nyx_semilla(unsigned long long s) {
    nyx_azar_estado = s ? s : 88172645463325252ULL;
}

static unsigned nyx_u32(void) {
    unsigned long long x = nyx_azar_estado;
    x ^= x << 13;
    x ^= x >> 7;
    x ^= x << 17;
    nyx_azar_estado = x;
    return (unsigned)(x >> 32);
}

static float nyx_f01(void) { return (float)(nyx_u32() >> 8) * (1.0f / 16777216.0f); }
static float nyx_rango(float a, float b) { return a + (b - a) * nyx_f01(); }
static int nyx_ent(int n) { return n <= 0 ? 0 : (int)(nyx_u32() % (unsigned)n); }
static float nyx_min(float a, float b) { return a < b ? a : b; }
static float nyx_max(float a, float b) { return a > b ? a : b; }
static float nyx_lim(float x, float a, float b) { return x < a ? a : (x > b ? b : x); }

static unsigned nyx_fnv(const char *s) {
    unsigned h = 2166136261u;
    while (*s) { h ^= (unsigned char)*s++; h *= 16777619u; }
    return h;
}

static void nyx_copia(char *dst, const char *src, size_t cap) {
    size_t i = 0;
    if (cap == 0) return;
    while (src[i] && i + 1 < cap) { dst[i] = src[i]; i++; }
    /* no cortar un carácter UTF-8 por la mitad */
    while (i > 0 && ((unsigned char)dst[i - 1] & 0xC0) == 0x80) {
        size_t j = i - 1;
        while (j > 0 && ((unsigned char)dst[j] & 0xC0) == 0x80) j--;
        {
            unsigned char c = (unsigned char)dst[j];
            size_t largo = c >= 0xF0 ? 4 : c >= 0xE0 ? 3 : c >= 0xC0 ? 2 : 1;
            if (j + largo <= i) break;
            i = j;
        }
    }
    dst[i] = 0;
}

/* ------------------------------------------------------------------ */
/*  Roles: cómo es cada una de las 18                                  */
/* ------------------------------------------------------------------ */

enum { ACC_HABLAR, ACC_PREGUNTAR, ACC_IMAGINAR, ACC_SONAR, ACC_RECORDAR, ACC_ESCUCHAR };
static const char *NYX_ACC[NACC] = { "hablar", "preguntar", "imaginar", "soñar", "recordar", "escuchar" };

typedef struct {
    const char *nombre;
    float ruido, rigidez, A0;   /* η, λ, A₀ de la dinámica */
    float sabeResh;             /* qué parte del Resh sabe al nacer */
    const char *objetivo;       /* lo que persigue */
    float gusto[NACC];          /* gusto innato por cada acción */
    int color;                  /* color ANSI 256 para la UI */
} RolInfo;

static const RolInfo NYX_ROLES[NROLES] = {
    { "sintaxis",    0.02f, 1.2f,  0.65f, 0.45f, "frase bien formada orden claro estructura", {.5f,.2f,.1f,.4f,.3f,.3f},  39 },
    { "semantica",   0.05f, 0.8f,  0.60f, 0.40f, "significado claro que se entienda",          {.6f,.3f,.2f,.3f,.3f,.3f},  45 },
    { "logica",      0.01f, 1.4f,  0.70f, 0.30f, "valido sin contradiccion coherente",         {.4f,.3f,.1f,.6f,.2f,.3f},  33 },
    { "codigo",      0.03f, 1.0f,  0.65f, 0.30f, "preciso ejecutable correcto",                {.5f,.2f,.2f,.5f,.2f,.2f},  46 },
    { "creativo",    0.12f, 0.4f,  0.50f, 0.25f, "nuevo inesperado original posibilidad",      {.4f,.2f,.9f,.2f,.2f,.1f}, 213 },
    { "critico",     0.02f, 1.5f,  0.55f, 0.25f, "falla error objecion verifica",              {.7f,.4f,.1f,.2f,.2f,.3f}, 196 },
    { "memoria",     0.03f, 0.5f,  0.80f, 0.55f, "recuerdo relevante pasado patron",           {.3f,.2f,.1f,.5f,.9f,.3f}, 141 },
    { "percepcion",  0.06f, 0.7f,  0.60f, 0.25f, "observa describe forma color ritmo",         {.4f,.3f,.3f,.2f,.2f,.6f}, 214 },
    { "sintesis",    0.09f, 0.5f,  0.55f, 0.30f, "integra une resume concluye",                {.6f,.2f,.5f,.5f,.3f,.2f},  82 },
    { "intuicion",   0.14f, 0.3f,  0.55f, 0.20f, "rapido corazonada destello ahora",           {.3f,.2f,.7f,.2f,.2f,.4f}, 219 },
    { "analogia",    0.11f, 0.45f, 0.55f, 0.25f, "puente entre dominios como metafora",        {.5f,.2f,.7f,.3f,.2f,.2f}, 117 },
    { "contexto",    0.04f, 0.9f,  0.70f, 0.35f, "hilo situacion aqui ahora marco",            {.4f,.3f,.1f,.3f,.5f,.5f}, 186 },
    { "esceptico",   0.03f, 1.3f,  0.55f, 0.25f, "duda pregunta cuestiona verifica",           {.4f,.8f,.1f,.3f,.2f,.3f}, 167 },
    { "narrativa",   0.06f, 0.6f,  0.60f, 0.35f, "secuencia entonces despues historia",        {.6f,.1f,.3f,.2f,.8f,.2f}, 179 },
    { "etica",       0.01f, 1.6f,  0.65f, 0.35f, "justo bien deber correcto",                  {.5f,.3f,.1f,.5f,.3f,.4f},  77 },
    { "curiosidad",  0.10f, 0.4f,  0.55f, 0.20f, "novedad explora descubre pregunta",          {.3f,.9f,.5f,.2f,.2f,.3f}, 226 },
    { "abstraccion", 0.05f, 0.7f,  0.75f, 0.30f, "esencia general eleva patron",               {.4f,.2f,.6f,.6f,.2f,.2f}, 105 },
    { "empatia",     0.05f, 0.6f,  0.65f, 0.35f, "escucha comprende alinea acompana",          {.5f,.3f,.2f,.2f,.3f,.8f}, 210 },
};

static int nyx_rol_de(const char *nombre) {
    int r;
    for (r = 0; r < NROLES; r++)
        if (strcmp(NYX_ROLES[r].nombre, nombre) == 0) return r;
    /* también vale el comienzo: "crea" -> creativo */
    for (r = 0; r < NROLES; r++)
        if (strlen(nombre) >= 3 && strncmp(NYX_ROLES[r].nombre, nombre, strlen(nombre)) == 0) return r;
    return -1;
}

/* Lo que leen al nacer: la infancia de las 18 (si no hay memoria guardada). */
static const char *NYX_CORPUS[] = {
    "la mente aprende cuando escucha y pregunta",
    "una idea nueva nace cuando dos ideas se unen",
    "la luz del sol hace crecer el árbol",
    "el agua corre por el río hasta el mar",
    "cada pregunta abre un camino nuevo",
    "el error enseña más que el acierto",
    "una historia tiene un principio y un final",
    "el recuerdo guarda lo que fue importante",
    "la duda ayuda a buscar la verdad",
    "un buen código es claro y correcto",
    "la palabra justa dice mucho con poco",
    "el amigo escucha y comprende",
    "la música tiene ritmo y forma",
    "el niño juega y descubre el mundo",
    "la noche trae silencio y estrellas",
    "pensar es unir lo que parece separado",
    "el tiempo pasa y la memoria queda",
    "un patrón se repite en muchas cosas",
    "la verdad se busca con preguntas",
    "el bien y el deber guían lo justo",
    "el color del cielo cambia con la luz",
    "la ciencia observa mide y explica",
    "un problema grande se divide en partes pequeñas",
    "la imaginación ve lo que todavía no existe",
    "el lenguaje une a las mentes",
    "aprender es cambiar con lo que se vive",
    "la casa protege del frío y de la lluvia",
    "el camino largo empieza con un paso",
    "la historia del mundo está llena de cambios",
    "el sueño ordena lo que pasó en el día",
    "una buena razón convence sin gritar",
    "la naturaleza tiene árboles ríos y montañas",
    "el número cuenta y la palabra explica",
    "escuchar al otro es una forma de respeto",
    "la curiosidad empuja a explorar",
    "lo simple es más fuerte que lo complicado",
    "el fuego da calor y luz",
    "cada mente piensa de forma distinta",
    "juntas las mentes ven más lejos",
    "la esencia de algo es lo que no cambia",
};
#define NYX_NCORPUS ((int)(sizeof NYX_CORPUS / sizeof NYX_CORPUS[0]))

/* Palabras vacías: sirven para la gramática, nunca para ganar un veredicto. */
static const char *NYX_VACIAS[] = {
    "el", "la", "los", "las", "un", "una", "unos", "unas", "de", "del", "al", "a", "y", "o", "u", "e",
    "que", "en", "por", "con", "para", "se", "su", "sus", "lo", "le", "les", "es", "son", "no", "mi",
    "tu", "me", "te", "nos", "ya", "más", "mas", "muy", "pero", "si", "sí", "como", "cuando", "hay",
    "este", "esta", "esto", "ese", "esa", "eso", "fue", "ser", "está", "están", "qué", "cómo", "yo",
    "tú", "él", "ella", "todo", "toda", "sin", "sobre", "entre", "hasta", "desde", "también", "ni",
};

static int nyx_vacia(const char *w) {
    size_t i;
    for (i = 0; i < sizeof NYX_VACIAS / sizeof NYX_VACIAS[0]; i++)
        if (strcmp(NYX_VACIAS[i], w) == 0) return 1;
    return 0;
}

static int nyx_conector(const char *w) {
    static const char *c[] = { "y", "o", "u", "e", "en", "por", "con", "de", "del", "que", "a", "al", "para", "ni", "pero", "sin", "como", "se", "es", "son" };
    size_t i;
    for (i = 0; i < sizeof c / sizeof c[0]; i++) if (strcmp(c[i], w) == 0) return 1;
    return 0;
}

/* ------------------------------------------------------------------ */
/*  Diccionario Resh (tabla hash sobre resh.h)                         */
/* ------------------------------------------------------------------ */

#define NYX_DICN 8192
static short nyx_dic_es[NYX_DICN], nyx_dic_re[NYX_DICN];
static int nyx_dic_listo = 0;

static const ParResh *nyx_par(int k) {
    return k < N_RESH_PROTOCOLO ? &RESH_PROTOCOLO[k] : &RESH_LEXICO[k - N_RESH_PROTOCOLO];
}

static void nyx_dic_pon(short *t, int k, int porResh) {
    const char *clave = porResh ? nyx_par(k)->resh : nyx_par(k)->es;
    unsigned h = nyx_fnv(clave) & (NYX_DICN - 1);
    while (t[h] >= 0) {
        const ParResh *p = nyx_par(t[h]);
        if (strcmp(porResh ? p->resh : p->es, clave) == 0) return;  /* el primero manda */
        h = (h + 1) & (NYX_DICN - 1);
    }
    t[h] = (short)k;
}

static void nyx_dic_init(void) {
    int k;
    if (nyx_dic_listo) return;
    for (k = 0; k < NYX_DICN; k++) { nyx_dic_es[k] = -1; nyx_dic_re[k] = -1; }
    for (k = 0; k < N_RESH_PROTOCOLO + N_RESH_LEXICO; k++) {
        nyx_dic_pon(nyx_dic_es, k, 0);
        nyx_dic_pon(nyx_dic_re, k, 1);
    }
    nyx_dic_listo = 1;
}

static const ParResh *nyx_dic_busca(const short *t, const char *clave, int porResh) {
    unsigned h = nyx_fnv(clave) & (NYX_DICN - 1);
    while (t[h] >= 0) {
        const ParResh *p = nyx_par(t[h]);
        if (strcmp(porResh ? p->resh : p->es, clave) == 0) return p;
        h = (h + 1) & (NYX_DICN - 1);
    }
    return NULL;
}

/* español -> resh (NULL si no está en el léxico) */
static const char *nyx_a_resh(const char *es) {
    const ParResh *p = nyx_dic_busca(nyx_dic_es, es, 0);
    return p ? p->resh : NULL;
}

/* resh -> español */
static const char *nyx_a_es(const char *resh) {
    const ParResh *p = nyx_dic_busca(nyx_dic_re, resh, 1);
    return p ? p->es : NULL;
}

static int nyx_es_particula(const char *es) {
    int k;
    for (k = 0; k < N_RESH_PROTOCOLO; k++)
        if (strcmp(RESH_PROTOCOLO[k].es, es) == 0) return 1;
    return 0;
}

/* ------------------------------------------------------------------ */
/*  Texto: palabras en minúscula, sin puntuación                       */
/* ------------------------------------------------------------------ */

static int nyx_tokens(const char *t, char toks[][LARGO], int max) {
    int n = 0;
    char w[64];
    int lw = 0;
    const unsigned char *p = (const unsigned char *)t;
    for (;;) {
        unsigned char c = *p;
        int corta = 0;
        if (c == 0) corta = 1;
        else if (c < 0x80) {
            if ((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
                if (lw < 60) w[lw++] = (char)c;
            } else if (c >= 'A' && c <= 'Z') {
                if (lw < 60) w[lw++] = (char)(c + 32);
            } else corta = 1;
            p++;
        } else if (c == 0xE2 && p[1] && p[2]) {
            corta = 1;            /* ◀ ▶ ⊕ … — símbolos, no letras */
            p += 3;
        } else if (c == 0xC2 && (p[1] == 0xBF || p[1] == 0xA1 || p[1] == 0xAB || p[1] == 0xBB)) {
            corta = 1;            /* ¿ ¡ « » */
            p += 2;
        } else if (c == 0xC3 && p[1] >= 0x80 && p[1] <= 0x9E && p[1] != 0x97) {
            if (lw < 59) { w[lw++] = (char)c; w[lw++] = (char)(p[1] + 0x20); }  /* Á -> á */
            p += 2;
        } else {
            if (lw < 60) w[lw++] = (char)c;
            p++;
        }
        if (corta || lw >= 60) {
            if (lw > 0 && n < max) {
                w[lw] = 0;
                nyx_copia(toks[n++], w, LARGO);
            }
            lw = 0;
            if (c == 0) break;
        }
    }
    return n;
}

/* Firma formal de una palabra (sin embeddings): forma, ritmo y símbolos. */
static void nyx_firma(const char *w, float *f) {
    int len = 0, nb = 0, sim = 0, dig = 0, voc = 0, simb = 0, acen = 0, i;
    double suma = 0, var = 0, media;
    int L = (int)strlen(w);
    for (i = 0; i < L; i++) {
        unsigned char c = (unsigned char)w[i];
        if ((c & 0xC0) != 0x80) len++;
        suma += c; nb++;
        if (c >= '0' && c <= '9') dig++;
        if (strchr("aeiou", c) && c) voc++;
        if (strchr("+-*/=<>^", c) && c) simb++;
        if (c >= 0x80) acen++;
    }
    media = nb ? suma / nb : 0;
    for (i = 0; i < L; i++) { double d = (unsigned char)w[i] - media; var += d * d; }
    var = nb ? var / nb : 0;
    for (i = 0; i < L / 2; i++) if (w[i] == w[L - 1 - i]) sim++;
    if (len == 0) len = 1;
    f[0] = (float)len / 20.0f;
    f[1] = (float)fmod(media, 128.0) / 128.0f;
    f[2] = nyx_min(1.0f, (float)(var / 4000.0));
    f[3] = (float)sim / (float)len;
    f[4] = (float)dig / (float)len;
    f[5] = (float)acen / (float)(nb ? nb : 1);
    f[6] = (float)voc / (float)len;
    f[7] = (float)simb / (float)len;
}

/* ------------------------------------------------------------------ */
/*  Mente                                                              */
/* ------------------------------------------------------------------ */

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

static float m_precision(const Mente *m) {
    return m->intentos > 0 ? (float)m->aciertos / (float)m->intentos : 0.5f;
}

static void m_ventana(Mente *m, const char *texto) {
    nyx_copia(m->ven[m->iven], texto, LVEN);
    m->iven = (m->iven + 1) % NVEN;
    if (m->nven < NVEN) m->nven++;
}

static int m_busca(const Mente *m, const char *et) {
    unsigned h = nyx_fnv(et) & (HASHN - 1);
    while (m->hash[h] >= 0) {
        if (strcmp(m->s[m->hash[h]].et, et) == 0) return m->hash[h];
        h = (h + 1) & (HASHN - 1);
    }
    return -1;
}

static void m_hash_pon(Mente *m, int i) {
    unsigned h = nyx_fnv(m->s[i].et) & (HASHN - 1);
    while (m->hash[h] >= 0) h = (h + 1) & (HASHN - 1);
    m->hash[h] = (short)i;
}

static void m_hash_rehace(Mente *m) {
    int i;
    for (i = 0; i < HASHN; i++) m->hash[i] = -1;
    for (i = 0; i < m->n; i++) m_hash_pon(m, i);
}

static int m_nuevo(Mente *m, const char *et, int fusion) {
    Semion *s;
    const char *r;
    if (m->n >= MAXSEM) return -1;
    s = &m->s[m->n];
    memset(s, 0, sizeof *s);
    nyx_copia(s->et, et, LARGO);
    nyx_firma(s->et, s->f);
    s->A = 0.6f;
    s->fase = nyx_rango(0, NYX_2PI);
    s->frec = nyx_rango(0.8f, 1.2f);
    s->fusion = (unsigned char)fusion;
    s->carga = (signed char)(fusion ? 1 : 0);
    r = fusion ? NULL : nyx_a_resh(s->et);
    if (r) s->sabe = (unsigned char)(nyx_es_particula(s->et) || nyx_f01() < NYX_ROLES[m->rol].sabeResh);
    m_hash_pon(m, m->n);
    return m->n++;
}

static Acople *m_acople(Mente *m, int a, int b) {
    Semion *s = &m->s[a];
    int k;
    for (k = 0; k < s->nv; k++)
        if (s->v[k].j == b) return &s->v[k];
    return NULL;
}

/* Crea el acople a->b (si no cabe, reemplaza el más débil). */
static Acople *m_acople_nuevo(Mente *m, int a, int b, float w, float th) {
    Semion *s = &m->s[a];
    Acople *c;
    if (a == b) return NULL;
    if (s->nv < MAXVEC) {
        c = &s->v[s->nv++];
    } else {
        int k, peor = 0;
        for (k = 1; k < MAXVEC; k++)
            if (s->v[k].w + s->v[k].sec < s->v[peor].w + s->v[peor].sec) peor = k;
        if (s->v[peor].w + s->v[peor].sec > w) return NULL;
        c = &s->v[peor];
    }
    c->j = (short)b; c->w = w; c->th = th; c->sec = 0;
    return c;
}

static void m_fija(Mente *m, int a, int b, float w, float th) {
    Acople *c = m_acople(m, a, b);
    if (!c) c = m_acople_nuevo(m, a, b, w, th);
    if (c) { c->w = w; c->th = th; }
}

/* Refuerzo con saturación: repetir acerca a 1 sin pasarse. */
static void m_refuerza(Mente *m, int a, int b, float dw, float th, float dsec) {
    Acople *c;
    if (a == b) return;
    c = m_acople(m, a, b);
    if (!c) { c = m_acople_nuevo(m, a, b, dw, th); if (!c) return; c->sec = dsec; return; }
    c->w += dw * (1 - c->w);
    c->sec += dsec * (1 - c->sec);
}

static float m_coh(const Mente *m, int i) {
    const Semion *s = &m->s[i];
    float c = 0;
    int k;
    for (k = 0; k < s->nv; k++) {
        const Semion *o = &m->s[s->v[k].j];
        c += s->v[k].w * s->A * o->A * cosf(s->fase - o->fase - s->v[k].th);
    }
    return c;
}

static void m_envuelve(float *f) {
    *f = fmodf(*f, NYX_2PI);
    if (*f < 0) *f += NYX_2PI;
}

static void m_decae(Mente *m, int i, float ruido) {
    Semion *s = &m->s[i];
    float A = s->A;
    float dA = -m->rigidez * (A * A - m->A0 * m->A0) * A * 0.02f;
    s->A = nyx_lim(A + dA, 0.05f, 1.0f);
    s->fase += s->frec * 0.02f + nyx_rango(-ruido, ruido);
    m_envuelve(&s->fase);
}

static void m_propaga(Mente *m, int i) {
    Semion *s = &m->s[i];
    float d = 0;
    int k;
    for (k = 0; k < s->nv; k++) {
        const Acople *c = &s->v[k];
        const Semion *o = &m->s[c->j];
        if (o->carga != s->carga && c->w <= 0.9f) continue;
        d += c->w * o->A * sinf(o->fase - s->fase - c->th);
    }
    s->fase += 0.05f * d;
    m_envuelve(&s->fase);
}

/* Lo que el estímulo despierta: él, sus vecinos y los vecinos de esos. */
static int m_region(const Mente *m, const int *est, int ne, int *out, int tope) {
    static unsigned char marca[MAXSEM];
    int n = 0, i, k, ini, fin;
    for (i = 0; i < ne && n < tope; i++)
        if (est[i] >= 0 && !marca[est[i]]) { marca[est[i]] = 1; out[n++] = est[i]; }
    ini = 0; fin = n;
    for (i = ini; i < fin && n < tope; i++) {
        const Semion *s = &m->s[out[i]];
        for (k = 0; k < s->nv && n < tope; k++)
            if (!marca[s->v[k].j]) { marca[s->v[k].j] = 1; out[n++] = s->v[k].j; }
    }
    ini = fin; fin = n;
    for (i = ini; i < fin && n < tope; i++) {
        const Semion *s = &m->s[out[i]];
        for (k = 0; k < s->nv && k < 12 && n < tope; k++)
            if (!marca[s->v[k].j]) { marca[s->v[k].j] = 1; out[n++] = s->v[k].j; }
    }
    for (i = 0; i < n; i++) marca[out[i]] = 0;
    return n;
}

static void m_atiende(Mente *m, int id) {
    int i, j = 0;
    for (i = 0; i < m->nfoco; i++) if (m->foco[i] != id) m->foco[j++] = m->foco[i];
    m->nfoco = j;
    if (m->nfoco == 7) { memmove(m->foco, m->foco + 1, 6 * sizeof m->foco[0]); m->nfoco = 6; }
    m->foco[m->nfoco++] = (short)id;
    m->s[id].A = nyx_min(1, m->s[id].A + 0.12f);
}

static void m_episodio(Mente *m, int id) {
    if (m->nepis == 24) { memmove(m->epis, m->epis + 1, 23 * sizeof m->epis[0]); m->nepis = 23; }
    m->epis[m->nepis++] = (short)id;
}

static int m_es_objetivo(const Mente *m, int id) {
    int k;
    for (k = 0; k < m->nobj; k++) if (m->obj[k] == id) return 1;
    return 0;
}

/* Poda homeostática: libera ~12% cuando la mente se llena. Primero caen las
 * ideas propias que nadie retomó; luego lo menos coherente y usado. */
static void m_poda(Mente *m) {
    static float punt[MAXSEM];
    static short nuevo[MAXSEM];
    int quitar = m->n / 8, i, k, q;
    for (i = 0; i < m->n; i++) {
        const Semion *s = &m->s[i];
        punt[i] = s->A + 0.08f * (float)s->usos + 0.3f * (float)s->dialogo + 0.2f * m_coh(m, i)
                + 0.03f * (float)s->nv + (s->sabe ? 0.4f : 0.0f);
        if (s->fusion && s->dialogo == 0) punt[i] -= 5;
        if (m_es_objetivo(m, i)) punt[i] += 1000;
    }
    for (i = 0; i < m->n; i++) nuevo[i] = 0;
    for (q = 0; q < quitar; q++) {           /* marca los 'quitar' peores */
        int peor = -1;
        for (i = 0; i < m->n; i++)
            if (!nuevo[i] && (peor < 0 || punt[i] < punt[peor])) peor = i;
        if (peor < 0) break;
        nuevo[peor] = -1;
    }
    k = 0;
    for (i = 0; i < m->n; i++) {
        if (nuevo[i] == -1) continue;
        nuevo[i] = (short)k;
        if (k != i) m->s[k] = m->s[i];
        k++;
    }
    {
        int nn = k, a;
        for (a = 0; a < nn; a++) {
            Semion *s = &m->s[a];
            int j = 0;
            for (i = 0; i < s->nv; i++) {
                short d = nuevo[s->v[i].j];
                if (d >= 0) { s->v[j] = s->v[i]; s->v[j].j = d; j++; }
            }
            s->nv = (unsigned char)j;
        }
        /* índices guardados en otras partes */
        for (i = 0, k = 0; i < m->nobj; i++) if (nuevo[m->obj[i]] >= 0) m->obj[k++] = nuevo[m->obj[i]];
        m->nobj = k;
        for (i = 0, k = 0; i < m->nfoco; i++) if (nuevo[m->foco[i]] >= 0) m->foco[k++] = nuevo[m->foco[i]];
        m->nfoco = k;
        for (i = 0, k = 0; i < m->nrec; i++) if (nuevo[m->recientes[i]] >= 0) m->recientes[k++] = nuevo[m->recientes[i]];
        m->nrec = k;
        for (i = 0, k = 0; i < m->nepis; i++) if (nuevo[m->epis[i]] >= 0) m->epis[k++] = nuevo[m->epis[i]];
        m->nepis = k;
        for (i = 0, k = 0; i < m->ncam; i++) {
            Camino c = m->cam[i];
            if (nuevo[c.a] >= 0 && nuevo[c.b] >= 0 && nuevo[c.c] >= 0) {
                c.a = nuevo[c.a]; c.b = nuevo[c.b]; c.c = nuevo[c.c];
                m->cam[k++] = c;
            }
        }
        m->ncam = k;
        m->n = nn;
    }
    m_hash_rehace(m);
}

/* Ingesta: texto -> semiones, con asociación (ventana de 3) y secuencia. */
static int m_ingesta2(Mente *m, const char *texto, int *ids, int max, int dialogo, float gramatica);

/* Ingesta con gramática plena (lo que escribe el humano o un libro). */
static int m_ingesta(Mente *m, const char *texto, int *ids, int max, int dialogo) {
    return m_ingesta2(m, texto, ids, max, dialogo, 1.0f);
}

/* gramatica: cuánto se aprende el ORDEN de las palabras. Las frases de otras
 * mentes enseñan asociaciones, pero poca gramática: si no, se copiarían sus
 * frases a medio hacer y el idioma se degradaría. */
static int m_ingesta2(Mente *m, const char *texto, int *ids, int max, int dialogo, float gramatica) {
    char toks[MAXEST][LARGO];
    int nt = nyx_tokens(texto, toks, MAXEST), k = 0, i, d;
    if (m->n > MAXSEM - MAXEST - 8) m_poda(m);
    for (i = 0; i < nt && k < max; i++) {
        int id = m_busca(m, toks[i]);
        if (id < 0) id = m_nuevo(m, toks[i], 0);
        if (id < 0) continue;
        m->s[id].A = nyx_min(1, m->s[id].A + 0.15f);
        if (m->s[id].usos < 65000) m->s[id].usos++;
        if (dialogo && m->s[id].dialogo < 65000) m->s[id].dialogo++;
        ids[k++] = id;
    }
    for (i = 0; i < k; i++)
        for (d = 1; d <= 3 && i + d < k; d++) {
            if (ids[i] == ids[i + d]) continue;
            m_refuerza(m, ids[i], ids[i + d], 0.35f / (float)d, 0.3f, d == 1 ? 0.4f * gramatica : 0.0f);
            m_refuerza(m, ids[i + d], ids[i], 0.2f / (float)d, -0.3f, 0.0f);
        }
    return k;
}

/* Predicción: lo que espera oír según su foco. */
static int m_predice(const Mente *m, int *out, int max) {
    int cand[64], n = 0, i, k;
    float peso[64];
    for (i = 0; i < m->nfoco; i++) {
        const Semion *s = &m->s[m->foco[i]];
        for (k = 0; k < s->nv; k++) {
            int j = s->v[k].j, x;
            float p = s->v[k].w * m->s[j].A;
            for (x = 0; x < n; x++) if (cand[x] == j) break;
            if (x == n) { if (n == 64) continue; cand[n] = j; peso[n] = 0; n++; }
            peso[x] += p;
        }
    }
    for (i = 0; i < max && i < n; i++) {
        int mejor = i;
        for (k = i + 1; k < n; k++) if (peso[k] > peso[mejor]) mejor = k;
        { int t = cand[i]; float tp = peso[i]; cand[i] = cand[mejor]; peso[i] = peso[mejor]; cand[mejor] = t; peso[mejor] = tp; }
        out[i] = cand[i];
    }
    return i;
}

/* Recibir: transducir lo que otro dijo. Lo sorprendente se aprende más. */
static int m_recibe(Mente *m, const char *texto, const char *quien, int *ids, int max) {
    int pred[3], np = m_predice(m, pred, 3), k, i, aciertos = 0;
    float sorpresa;
    char linea[LVEN];
    k = m_ingesta2(m, texto, ids, max, 1, strcmp(quien, "humano") == 0 ? 1.0f : 0.15f);
    for (i = 0; i < k; i++) {
        int x;
        for (x = 0; x < np; x++) if (pred[x] == ids[i]) aciertos++;
    }
    sorpresa = np == 0 ? 0.5f : 1.0f - (float)aciertos / (float)(k ? k : 1);
    for (i = 0; i < k; i++) {
        Semion *s = &m->s[ids[i]];
        s->A = nyx_min(1, s->A + 0.25f * sorpresa + 0.05f);
        if (s->A > 0.55f && !nyx_vacia(s->et)) m_atiende(m, ids[i]);
    }
    snprintf(linea, sizeof linea, "◀ %s: %s", quien, texto);
    m_ventana(m, linea);
    return k;
}

/* Elige el siguiente vecino más coherente no visitado (para razonar a saltos). */
static int m_salto(const Mente *m, int id, const int *vis, int nvis) {
    const Semion *s = &m->s[id];
    int k, mejor = -1;
    float mc = -1e9f;
    for (k = 0; k < s->nv; k++) {
        int j = s->v[k].j, x, ya = 0;
        float c;
        if (m->s[j].fusion || nyx_vacia(m->s[j].et)) continue;
        for (x = 0; x < nvis; x++) if (vis[x] == j) ya = 1;
        if (ya) continue;
        c = m_coh(m, j);
        if (c > mc) { mc = c; mejor = j; }
    }
    return mejor;
}

/* Cristalización: un camino A->B->C recorrido 3 veces se vuelve concepto. */
static int m_cristaliza(Mente *m, int a, int b, int c) {
    char et[LARGO * 3];
    char ea[10], eb[10], ec[10];
    int id, k;
    if (m->n >= MAXSEM - 1) return -1;
    nyx_copia(ea, m->s[a].et, sizeof ea);
    nyx_copia(eb, m->s[b].et, sizeof eb);
    nyx_copia(ec, m->s[c].et, sizeof ec);
    snprintf(et, sizeof et, "%s⊕%s⊕%s", ea, eb, ec);
    nyx_copia(et, et, LARGO);
    if (m_busca(m, et) >= 0) return -1;
    id = m_nuevo(m, et, 1);
    if (id < 0) return -1;
    m->s[id].A = 0.9f;
    m->s[id].dialogo = 2;
    for (k = 0; k < DIMF; k++) m->s[id].f[k] = (m->s[a].f[k] + m->s[b].f[k] + m->s[c].f[k]) / 3;
    m_fija(m, a, id, 0.8f, 0); m_fija(m, id, a, 0.8f, 0);
    m_fija(m, b, id, 0.8f, 0); m_fija(m, id, b, 0.8f, 0);
    m_fija(m, c, id, 0.8f, 0); m_fija(m, id, c, 0.8f, 0);
    m->cristales++;
    return id;
}

/* Inferencia transitiva: refuerza A->B->C; el atajo perdura. */
static int m_transitiva(Mente *m, int id) {
    int vis[3], s1, s2, k;
    Acople *c;
    vis[0] = id;
    s1 = m_salto(m, id, vis, 1);
    if (s1 < 0) return -1;
    c = m_acople(m, id, s1);
    m_fija(m, id, s1, nyx_min(1, (c ? c->w : 0.3f) + 0.1f), 0.2f);
    vis[1] = s1;
    s2 = m_salto(m, s1, vis, 2);
    if (s2 < 0) return -1;
    c = m_acople(m, s1, s2);
    m_fija(m, s1, s2, nyx_min(1, (c ? c->w : 0.3f) + 0.1f), 0.2f);
    for (k = 0; k < m->ncam; k++) {
        Camino *p = &m->cam[k];
        if (p->a == id && p->b == s1 && p->c == s2) {
            if (++p->k == 3) return m_cristaliza(m, id, s1, s2);
            return -1;
        }
    }
    if (m->ncam == NCAM) { memmove(m->cam, m->cam + 1, (NCAM - 1) * sizeof m->cam[0]); m->ncam--; }
    m->cam[m->ncam].a = (short)id; m->cam[m->ncam].b = (short)s1; m->cam[m->ncam].c = (short)s2; m->cam[m->ncam].k = 1;
    m->ncam++;
    return -1;
}

static float m_incrustacion(const Mente *m, int id) {
    const Semion *s = &m->s[id];
    int k, n = 0;
    if (s->nv == 0) return 0;
    for (k = 0; k < s->nv; k++) if (m->s[s->v[k].j].dialogo > 0) n++;
    return (float)n / (float)s->nv;
}

/* Veredicto: el estímulo despierta una región, se relaja con recocido
 * (caliente -> frío) y gana lo que la pregunta EVOCA (no su eco). */
static int m_veredicto(Mente *m, const int *est0, int ne0, float *C, float *E, int *cristal) {
    static int reg[320];
    int est[MAXEST], ne = 0, nr, it, i, gan = -1;
    float mejor = -1e9f;
    if (cristal) *cristal = -1;
    /* lo que importa de la pregunta son sus palabras con contenido:
     * "qué", "es", "la" están en todas partes y llevarían a cualquier lado */
    for (i = 0; i < ne0 && ne < MAXEST; i++) if (!nyx_vacia(m->s[est0[i]].et)) est[ne++] = est0[i];
    if (ne == 0) for (i = 0; i < ne0 && ne < MAXEST; i++) est[ne++] = est0[i];
    nr = m_region(m, est, ne, reg, 300);
    if (nr == 0) { *C = 0; *E = 0; return -1; }
    for (it = 0; it < 60; it++) {
        float r = m->ruido * (3.0f - 2.0f * (float)it / 60.0f);
        for (i = 0; i < nr; i++) m_decae(m, reg[i], r);
        for (i = 0; i < nr; i++) m_propaga(m, reg[i]);
    }
    for (i = 0; i < nr; i++) {
        int id = reg[i], x, esEst = 0;
        float enlace = 0, rel;
        const Semion *s = &m->s[id];
        if (s->fusion || nyx_vacia(s->et)) continue;
        for (x = 0; x < ne; x++) {
            Acople *c;
            if (est[x] == id) esEst = 1;
            /* un acople inhibido (θ≈π) resta: así 'mal' cambia la respuesta */
            c = m_acople(m, est[x], id); if (c) enlace += c->w * cosf(c->th);
            c = m_acople(m, id, est[x]); if (c) enlace += c->w * cosf(c->th);
        }
        if (esEst && nr > ne) continue;
        /* manda lo que la pregunta evoca (enlace); la coherencia desempata */
        rel = enlace + 0.35f * tanhf(m_coh(m, id)) + 0.1f * (float)(m_es_objetivo(m, id));
        /* lo muy frecuente pesa menos: no gana lo que sale en todas partes */
        rel /= 1.0f + logf(1.0f + (float)s->usos) * 0.3f;
        {   /* fatiga: lo que acaba de ganar descansa un poco */
            int q;
            for (q = 0; q < m->nrec; q++) if (m->recientes[q] == id) { rel -= (rel > 0 ? rel : -rel) + 0.2f; break; }
        }
        if (rel > mejor) { mejor = rel; gan = id; }
    }
    if (gan < 0) {      /* solo estaba el propio estímulo */
        for (i = 0; i < ne; i++) if (!nyx_vacia(m->s[est[i]].et)) { gan = est[i]; break; }
        if (gan < 0) gan = est[0];
    }
    {
        int cr = m_transitiva(m, gan);
        if (cristal) *cristal = cr;
    }
    m_episodio(m, gan);
    m_atiende(m, gan);
    if (m->nrec == 6) { memmove(m->recientes, m->recientes + 1, 5 * sizeof m->recientes[0]); m->nrec = 5; }
    m->recientes[m->nrec++] = (short)gan;
    *C = m_coh(m, gan);
    *E = m_incrustacion(m, gan);
    return gan;
}

/* Frase: lo que va antes y después del centro según la memoria de
 * secuencia. Devuelve los semiones en orden. */
static float m_en_contexto(const int *ctx, int nctx, int id) {
    int i;
    for (i = 0; i < nctx; i++) if (ctx[i] == id) return 0.6f;
    return 0;
}

/* ctx: las palabras de lo que se preguntó; la frase prefiere ir por ahí
 * (así "qué es el mar" lleva a "el mar es grande", no a otra frase con "es"). */
static int m_frase(Mente *m, int centro, int *out, int max, const int *ctx, int nctx) {
    int pre[3], npre = 0, n = 0, i, k, actual, objetivoLargo;
    /* hasta 2 palabras que suelen ir ANTES */
    actual = centro;
    while (npre < 2) {
        int mejor = -1;
        float mp = 0.04f;
        for (i = 0; i < m->n; i++) {
            Acople *c;
            int ya = 0, x;
            if (i == actual || m->s[i].fusion) continue;
            c = m_acople(m, i, actual);
            if (!c || c->sec <= 0) continue;
            for (x = 0; x < npre; x++) if (pre[x] == i) ya = 1;
            if (ya || i == centro) continue;
            {
                float p = c->sec * (0.5f + m->s[i].A) + m_en_contexto(ctx, nctx, i) + nyx_rango(0, m->ruido * 2);
                if (p > mp) { mp = p; mejor = i; }
            }
        }
        if (mejor < 0) break;
        pre[npre++] = mejor;
        actual = mejor;
    }
    /* no empezar con "y", "en", "por"… (los artículos sí valen) */
    while (npre > 0 && nyx_conector(m->s[pre[npre - 1]].et)) npre--;
    for (i = npre - 1; i >= 0 && n < max; i--) out[n++] = pre[i];
    out[n++] = centro;
    objetivoLargo = 4 + nyx_ent(5);
    actual = centro;
    while (n < max && n < objetivoLargo) {
        const Semion *s = &m->s[actual];
        int mejor = -1;
        float mp = 0.02f;
        for (k = 0; k < s->nv; k++) {
            int j = s->v[k].j, x, ya = 0;
            float p;
            if (s->v[k].sec <= 0 || m->s[j].fusion) continue;
            for (x = 0; x < n; x++) if (out[x] == j) ya = 1;
            if (ya) continue;
            p = s->v[k].sec * (0.5f + m->s[j].A) + m_en_contexto(ctx, nctx, j) + nyx_rango(0, m->ruido * 3);
            if (p > mp) { mp = p; mejor = j; }
        }
        if (mejor < 0) break;
        out[n++] = mejor;
        actual = mejor;
    }
    /* una frase no termina en "y", "por", "un"…: se quita lo que cuelga */
    while (n > 2 && nyx_vacia(m->s[out[n - 1]].et) && out[n - 1] != centro) n--;
    if (n < 2) {      /* sin secuencia: al menos su asociación más fuerte */
        const Semion *s = &m->s[centro];
        int mejor = -1;
        float mw = 0;
        for (k = 0; k < s->nv; k++)
            if (!m->s[s->v[k].j].fusion && s->v[k].w > mw) { mw = s->v[k].w; mejor = s->v[k].j; }
        if (mejor >= 0 && n < max) out[n++] = mejor;
    }
    return n;
}

static void m_texto(const Mente *m, const int *ids, int n, char *es, size_t ces, char *resh, size_t cre) {
    int i;
    size_t le = 0, lr = 0;
    es[0] = 0; resh[0] = 0;
    for (i = 0; i < n; i++) {
        const Semion *s = &m->s[ids[i]];
        const char *r = s->sabe ? nyx_a_resh(s->et) : NULL;
        le += (size_t)snprintf(es + le, le < ces ? ces - le : 0, "%s%s", i ? " " : "", s->et);
        lr += (size_t)snprintf(resh + lr, lr < cre ? cre - lr : 0, "%s%s", i ? " " : "", r ? r : s->et);
        if (le >= ces) le = ces - 1;
        if (lr >= cre) lr = cre - 1;
    }
}

/* Idea propia: fusiona dos ideas activas parecidas por su firma efectiva. */
static int m_imagina(Mente *m, int *pa, int *pb) {
    int act[64], na = 0, i, k, a, b = -1, id;
    float fa[DIMF], mejor = 1e9f;
    char et[LARGO * 2], ea[12], eb[12];
    if (m->n > MAXSEM - 4) m_poda(m);
    for (k = 0; k < 400 && na < 64; k++) {
        int x = nyx_ent(m->n);
        if (m->n == 0) break;
        if (m->s[x].A > 0.45f && !m->s[x].fusion && !nyx_vacia(m->s[x].et)) {
            int y, ya = 0;
            for (y = 0; y < na; y++) if (act[y] == x) ya = 1;
            if (!ya) act[na++] = x;
        }
    }
    if (na < 2) return -1;
    a = act[nyx_ent(na)];
    for (k = 0; k < DIMF; k++) fa[k] = m->s[a].f[k];
    for (i = 0; i < na; i++) {
        float d = 0;
        if (act[i] == a) continue;
        for (k = 0; k < DIMF; k++) { float e = fa[k] - m->s[act[i]].f[k]; d += e * e; }
        /* además de la forma: si ya comparten vecinos, se acercan */
        {
            const Semion *sa = &m->s[a];
            int x;
            for (x = 0; x < sa->nv; x++) if (m_acople(m, act[i], sa->v[x].j)) d -= 0.05f;
        }
        d += nyx_rango(0, m->ruido);
        if (d < mejor) { mejor = d; b = act[i]; }
    }
    if (b < 0) return -1;
    nyx_copia(ea, m->s[a].et, sizeof ea);
    nyx_copia(eb, m->s[b].et, sizeof eb);
    snprintf(et, sizeof et, "%s⊕%s", ea, eb);
    nyx_copia(et, et, LARGO);
    id = m_busca(m, et);
    if (id < 0) id = m_nuevo(m, et, 1);
    if (id < 0) return -1;
    for (k = 0; k < DIMF; k++) m->s[id].f[k] = (m->s[a].f[k] + m->s[b].f[k]) / 2;
    m->s[id].fase = (m->s[a].fase + m->s[b].fase) / 2;
    m->s[id].A = nyx_min(1, (m->s[a].A + m->s[b].A) / 2 + 0.1f);
    m_fija(m, a, id, 0.5f, 0.1f);
    m_fija(m, b, id, 0.5f, -0.1f);
    m_fija(m, id, a, 0.5f, 0.1f);
    m_fija(m, id, b, 0.5f, -0.1f);
    m_refuerza(m, a, b, 0.2f, 0, 0);
    m->ideas++;
    *pa = a; *pb = b;
    return id;
}

/* Después de decir algo, esas ideas se cansan: el tema puede cambiar. */
static void m_cansa(Mente *m, const int *ids, int n) {
    int i;
    for (i = 0; i < n; i++) m->s[ids[i]].A = nyx_max(0.1f, m->s[ids[i]].A * 0.7f);
}

/* Un ciclo de pensamiento local: relaja lo que está en el foco. */
static void m_piensa(Mente *m) {
    int est[7], reg[200], nr, i;
    for (i = 0; i < m->nfoco; i++) est[i] = m->foco[i];
    nr = m_region(m, est, m->nfoco, reg, 160);
    for (i = 0; i < nr; i++) m_decae(m, reg[i], m->ruido);
    for (i = 0; i < nr; i++) m_propaga(m, reg[i]);
    for (i = 0; i < m->nobj; i++) m->s[m->obj[i]].A = nyx_min(1, m->s[m->obj[i]].A + 0.01f);
    m->ciclos++;
}

static void m_init(Mente *m, int rol) {
    int ids[MAXEST], n, i;
    memset(m, 0, sizeof *m);
    m->rol = rol;
    m->ruido = NYX_ROLES[rol].ruido;
    m->rigidez = NYX_ROLES[rol].rigidez;
    m->A0 = NYX_ROLES[rol].A0;
    for (i = 0; i < HASHN; i++) m->hash[i] = -1;
    for (i = 0; i < NACC; i++) m->valor[i] = 0;
    m->ultima = -1;
    n = m_ingesta(m, NYX_ROLES[rol].objetivo, ids, MAXEST, 0);
    for (i = 0; i < n && m->nobj < 8; i++) {
        m->obj[m->nobj++] = (short)ids[i];
        m->s[ids[i]].A = 0.95f;
        if (nyx_a_resh(m->s[ids[i]].et)) m->s[ids[i]].sabe = 1;   /* su objetivo lo sabe decir */
    }
}

/* ------------------------------------------------------------------ */
/*  Consejo: las 18 juntas                                             */
/* ------------------------------------------------------------------ */

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
} NyxRespuesta;

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
} Consejo;

static int c_es_eco(const Consejo *c, const char *es) {
    int i;
    for (i = 0; i < 10; i++) if (c->ecos[i][0] && strcmp(c->ecos[i], es) == 0) return 1;
    return 0;
}

static void c_pon_eco(Consejo *c, const char *es) {
    nyx_copia(c->ecos[c->iecos], es, sizeof c->ecos[0]);
    c->iecos = (c->iecos + 1) % 10;
}

static void c_evento(Consejo *c, int rol, int tipo, const char *texto) {
    if (c->ev) c->ev(c->ud, rol, tipo, texto);
}

static void consejo_init(Consejo *c, unsigned long long semilla, int conInfancia) {
    int r, i, ids[MAXEST];
    nyx_dic_init();
    nyx_semilla(semilla);
    memset(c, 0, sizeof *c);
    for (r = 0; r < NROLES; r++) m_init(&c->m[r], r);
    if (conInfancia)
        for (i = 0; i < NYX_NCORPUS; i++)
            for (r = 0; r < NROLES; r++) m_ingesta(&c->m[r], NYX_CORPUS[i], ids, MAXEST, 0);
    c->ultimo = -1;
}

/* Todas leen un texto (enseñar). Devuelve cuántas palabras nuevas hubo. */
static int consejo_lee(Consejo *c, const char *texto) {
    int r, ids[MAXEST], antes = c->m[0].n;
    for (r = 0; r < NROLES; r++) m_ingesta(&c->m[r], texto, ids, MAXEST, 0);
    return c->m[0].n - antes > 0 ? c->m[0].n - antes : 0;
}

/* Las demás oyen lo que dijo 'emisor'. Si el emisor sabía decir una palabra
 * en Resh y la oyente no, puede aprenderla. Devuelve la resonancia media. */
static float c_difunde(Consejo *c, int emisor, const int *frase, int nf, int *aprendieron, char *palabraAprendida) {
    Mente *e = &c->m[emisor];
    char es[400], resh[400];
    float suma = 0;
    int r, i, oyentes = 0;
    m_texto(e, frase, nf, es, sizeof es, resh, sizeof resh);
    *aprendieron = 0;
    palabraAprendida[0] = 0;
    for (r = 0; r < NROLES; r++) {
        Mente *o;
        int ids[MAXEST], k;
        float res = 0;
        if (r == emisor) continue;
        o = &c->m[r];
        k = m_recibe(o, es, NYX_ROLES[emisor].nombre, ids, MAXEST);
        for (i = 0; i < k; i++) {
            res += m_coh(o, ids[i]);
            if (i < nf && e->s[frase[i]].sabe && !o->s[ids[i]].sabe && nyx_a_resh(o->s[ids[i]].et)
                && nyx_f01() < 0.35f) {
                o->s[ids[i]].sabe = 1;
                o->reshAprendidas++;
                /* se aprenden todas, pero solo se anuncian las que dicen algo */
                if (!nyx_vacia(o->s[ids[i]].et)) {
                    (*aprendieron)++;
                    nyx_copia(palabraAprendida, o->s[ids[i]].et, LARGO);
                }
            }
        }
        suma += k ? res / (float)k : 0;
        oyentes++;
    }
    return oyentes ? suma / (float)oyentes : 0;
}

/* ¿Ya lo dijo hace poco? (anti-eco) */
static int c_ya_dicho(const Mente *m, const char *es) {
    int i;
    for (i = 0; i < m->nven; i++)
        if (strncmp(m->ven[i], "dije: ", 6) == 0 && strcmp(m->ven[i] + 6, es) == 0) return 1;
    return 0;
}

/* Tema de esta mente ahora: lo más activo de su foco (o de sus objetivos). */
static int c_tema(const Mente *m) {
    int i, mejor = -1;
    float mA = -1;
    for (i = 0; i < m->nfoco; i++) {
        const Semion *s = &m->s[m->foco[i]];
        if (s->fusion || nyx_vacia(s->et)) continue;
        if (s->A + nyx_rango(0, 0.3f) > mA) { mA = s->A; mejor = m->foco[i]; }
    }
    if (mejor < 0 && m->nobj > 0) mejor = m->obj[nyx_ent(m->nobj)];
    return mejor;
}

/* Palabra que quiere aprender a decir en Resh (o -1). */
static int c_duda(const Mente *m) {
    int i, mejor = -1;
    float mA = 0.3f;
    for (i = 0; i < m->n; i++) {
        const Semion *s = &m->s[i];
        if (s->sabe || s->fusion || nyx_vacia(s->et) || !nyx_a_resh(s->et)) continue;
        if (s->A + 0.05f * (float)(s->usos > 10 ? 10 : s->usos) > mA) { mA = s->A; mejor = i; }
    }
    return mejor;
}

static void c_aprende(Consejo *c, int rol, int acc, float recompensa) {
    Mente *m = &c->m[rol];
    float antes = m->valor[acc];
    char t[128];
    m->valor[acc] += 0.25f * (recompensa - m->valor[acc]);
    m->veces[acc]++;
    snprintf(t, sizeof t, "'%s' ahora vale %.2f (%+.2f)", NYX_ACC[acc], m->valor[acc], m->valor[acc] - antes);
    c_evento(c, rol, EV_APRENDE, t);
}

static float c_puntaje(const Mente *m, int a) {
    float p = NYX_ROLES[m->rol].gusto[a] + m->valor[a] + 0.3f / (1.0f + (float)m->veces[a]);
    if (a == m->ultima) p -= 0.15f * (float)m->racha;   /* aburrimiento */
    return p + nyx_rango(-m->ruido * 2, m->ruido * 2);
}

/* Habla una mente: forma una frase desde 'centro', la dice, las demás la oyen. */
static float c_habla(Consejo *c, int rol, int centro, const char *prefijo) {
    Mente *m = &c->m[rol];
    int frase[MAXFRASE], nf, aprend;
    char es[400], resh[400], linea[900], palabra[LARGO];
    float res;
    {
        int ctx[7], i;
        for (i = 0; i < m->nfoco; i++) ctx[i] = m->foco[i];
        nf = m_frase(m, centro, frase, MAXFRASE, ctx, m->nfoco);
    }
    m_texto(m, frase, nf, es, sizeof es, resh, sizeof resh);
    if (c_ya_dicho(m, es) || c_es_eco(c, es)) return -0.6f;
    c_pon_eco(c, es);
    snprintf(linea, sizeof linea, "%s%s   «%s»", prefijo, resh, es);
    c_evento(c, rol, EV_DICE, linea);
    snprintf(linea, sizeof linea, "dije: %s", es);
    m_ventana(m, linea);
    m->dichos++;
    res = c_difunde(c, rol, frase, nf, &aprend, palabra);
    m_cansa(m, frase, nf);
    if (aprend > 0) {
        snprintf(linea, sizeof linea, "%d mente%s aprendi%s a decir «%s» = %s",
                 aprend, aprend > 1 ? "s" : "", aprend > 1 ? "eron" : "ó", palabra, nyx_a_resh(palabra));
        c_evento(c, rol, EV_RESH, linea);
    }
    return 1.5f * tanhf(res);
}

/* Un paso de vida libre: una mente imagina qué quiere hacer, lo hace,
 * mide cómo le fue y aprende. */
static void consejo_paso(Consejo *c) {
    int rol, a, tema, duda, mejorA = -1, segunda = -1, tercera = -1, posible[NACC], i;
    float punt[NACC], rec = 0;
    Mente *m;
    char t[400];
    c->tick++;
    do { rol = nyx_ent(NROLES); } while (rol == c->ultimo && NROLES > 1);
    c->ultimo = rol;
    m = &c->m[rol];
    for (i = 0; i < 3; i++) m_piensa(m);
    tema = c_tema(m);
    duda = c_duda(m);
    for (a = 0; a < NACC; a++) {
        posible[a] = 1;
        punt[a] = c_puntaje(m, a);
    }
    if (tema < 0) posible[ACC_HABLAR] = 0;
    if (m->nepis < 2) posible[ACC_RECORDAR] = 0;
    for (a = 0; a < NACC; a++) {
        if (!posible[a]) continue;
        if (mejorA < 0 || punt[a] > punt[mejorA]) { tercera = segunda; segunda = mejorA; mejorA = a; }
        else if (segunda < 0 || punt[a] > punt[segunda]) { tercera = segunda; segunda = a; }
        else if (tercera < 0 || punt[a] > punt[tercera]) tercera = a;
    }
    {   /* exploración: a veces prueba otra cosa */
        int total = 0;
        float explora;
        for (a = 0; a < NACC; a++) total += m->veces[a];
        explora = nyx_max(0.08f, 0.35f - 0.01f * (float)total);
        if (nyx_f01() < explora) {
            int x;
            do { x = nyx_ent(NACC); } while (!posible[x]);
            if (x != mejorA) { tercera = segunda; segunda = mejorA; mejorA = x; }
        }
    }
    a = mejorA;
    if (a == m->ultima) m->racha++; else { m->ultima = a; m->racha = 1; }
    {
        const char *tt = tema >= 0 ? m->s[tema].et : "algo";
        char alt[96];
        alt[0] = 0;
        if (segunda >= 0 && tercera >= 0)
            snprintf(alt, sizeof alt, " · también pensó: %s %.2f, %s %.2f", NYX_ACC[segunda], punt[segunda], NYX_ACC[tercera], punt[tercera]);
        else if (segunda >= 0)
            snprintf(alt, sizeof alt, " · también pensó: %s %.2f", NYX_ACC[segunda], punt[segunda]);
        switch (a) {
        case ACC_HABLAR:    snprintf(t, sizeof t, "quiero hablar de «%s»%s", tt, alt); break;
        case ACC_PREGUNTAR: snprintf(t, sizeof t, "quiero preguntar%s%s%s", duda >= 0 ? " cómo se dice «" : " algo", duda >= 0 ? m->s[duda].et : "", duda >= 0 ? "»" : ""); { size_t l = strlen(t); snprintf(t + l, sizeof t - l, "%s", alt); } break;
        case ACC_IMAGINAR:  snprintf(t, sizeof t, "quiero imaginar algo nuevo%s", alt); break;
        case ACC_SONAR:     snprintf(t, sizeof t, "quiero soñar y ordenar lo que viví%s", alt); break;
        case ACC_RECORDAR:  snprintf(t, sizeof t, "quiero contar lo que recuerdo%s", alt); break;
        default:            snprintf(t, sizeof t, "quiero escuchar%s", alt); break;
        }
        c_evento(c, rol, EV_PIENSA, t);
    }
    switch (a) {
    case ACC_HABLAR: {
        int modo = nyx_ent(10);
        const char *pre = "mi ko ";
        if (m_coh(m, tema) < 0.2f) pre = "ye ";
        else if ((rol == 5 || rol == 12) && modo < 5) pre = "ne ";
        rec = c_habla(c, rol, tema, pre);
        break;
    }
    case ACC_PREGUNTAR: {
        if (duda >= 0) {
            int r, quien = -1;
            float mejorP = -1;
            snprintf(t, sizeof t, "ye %s?   «¿cómo se dice %s?»", m->s[duda].et, m->s[duda].et);
            c_evento(c, rol, EV_DICE, t);
            for (r = 0; r < NROLES; r++) {
                int id;
                if (r == rol) continue;
                id = m_busca(&c->m[r], m->s[duda].et);
                if (id >= 0 && c->m[r].s[id].sabe && m_precision(&c->m[r]) + nyx_rango(0, 0.3f) > mejorP) {
                    mejorP = m_precision(&c->m[r]);
                    quien = r;
                }
            }
            if (quien >= 0) {
                const char *rr = nyx_a_resh(m->s[duda].et);
                int r2, extra = 0;
                snprintf(t, sizeof t, "%s ka %s   «%s se dice %s»", m->s[duda].et, rr, m->s[duda].et, rr);
                c_evento(c, quien, EV_DICE, t);
                m->s[duda].sabe = 1;
                m->reshAprendidas++;
                for (r2 = 0; r2 < NROLES; r2++) {
                    int id;
                    if (r2 == rol || r2 == quien) continue;
                    id = m_busca(&c->m[r2], m->s[duda].et);
                    if (id >= 0 && !c->m[r2].s[id].sabe && nyx_f01() < 0.5f) {
                        c->m[r2].s[id].sabe = 1; c->m[r2].reshAprendidas++; extra++;
                    }
                }
                snprintf(t, sizeof t, "aprendió «%s» = %s%s", m->s[duda].et, rr, extra ? " (y otras que oyeron también)" : "");
                c_evento(c, rol, EV_RESH, t);
                rec = 1.5f;
            } else {
                c_evento(c, rol, EV_NOTA, "nadie supo contestarle");
                rec = -0.3f;
            }
        } else if (tema >= 0) {
            /* sin dudas de idioma: pregunta por una idea y otra le responde */
            int r = nyx_ent(NROLES), ids[MAXEST], k, cr;
            float C, E;
            if (r == rol) r = (r + 1) % NROLES;
            snprintf(t, sizeof t, "ye %s?   «¿qué piensas de %s, %s?»", m->s[tema].et, m->s[tema].et, NYX_ROLES[r].nombre);
            c_evento(c, rol, EV_DICE, t);
            k = m_recibe(&c->m[r], m->s[tema].et, NYX_ROLES[rol].nombre, ids, MAXEST);
            if (k > 0) {
                int g = m_veredicto(&c->m[r], ids, k, &C, &E, &cr);
                if (g >= 0) rec = c_habla(c, r, g, "");
                rec = rec > 0 ? 1.0f : 0.1f;
            }
        } else rec = -0.2f;
        break;
    }
    case ACC_IMAGINAR: {
        int pa, pb, id = m_imagina(m, &pa, &pb), x;
        if (id >= 0) {
            float C;
            for (x = 0; x < 10; x++) { m_decae(m, id, m->ruido); m_propaga(m, id); m_propaga(m, pa); m_propaga(m, pb); }
            C = m_coh(m, id);
            snprintf(t, sizeof t, "imaginó «%s» (de %s y %s) · coherencia %.2f", m->s[id].et, m->s[pa].et, m->s[pb].et, C);
            c_evento(c, rol, EV_IDEA, t);
            m_atiende(m, id);
            rec = nyx_lim(C * 2.0f, -1, 1.5f);
        } else rec = -0.3f;
        break;
    }
    case ACC_SONAR: {
        int i2, ids[8], n = 0, cr = -1;
        float antes = 0, despues = 0;
        for (i2 = 0; i2 < m->nfoco; i2++) antes += m_coh(m, m->foco[i2]);
        for (i2 = 0; i2 < 12; i2++) m_piensa(m);
        /* repasa lo más hablado y razona a saltos desde ahí */
        for (i2 = 0; i2 < m->n && n < 8; i2++) {
            int x = nyx_ent(m->n);
            if (m->s[x].dialogo > 0 && !m->s[x].fusion && !nyx_vacia(m->s[x].et)) ids[n++] = x;
        }
        for (i2 = 0; i2 < n; i2++) {
            m->s[ids[i2]].A = nyx_min(1, m->s[ids[i2]].A + 0.08f);
            if (cr < 0) cr = m_transitiva(m, ids[i2]);
        }
        for (i2 = 0; i2 < m->nfoco; i2++) despues += m_coh(m, m->foco[i2]);
        if (cr >= 0) {
            snprintf(t, sizeof t, "soñando cristalizó un concepto: «%s»", m->s[cr].et);
            c_evento(c, rol, EV_IDEA, t);
        } else {
            snprintf(t, sizeof t, "soñó con %d recuerdos · coherencia %+.2f", n, despues - antes);
            c_evento(c, rol, EV_SUENA, t);
        }
        rec = nyx_lim((despues - antes) * 0.5f + (cr >= 0 ? 1.0f : 0.1f), -1, 1.5f);
        break;
    }
    case ACC_RECORDAR: {
        int ids[MAXFRASE], n = 0, i2;
        for (i2 = m->nepis - 1; i2 >= 0 && n < 3; i2--) {
            int x, ya = 0;
            for (x = 0; x < n; x++) if (ids[x] == m->epis[i2]) ya = 1;
            if (!ya && !m->s[m->epis[i2]].fusion) ids[n++] = m->epis[i2];
        }
        if (n >= 2) {
            char es[400], resh[400], linea[900];
            int aprend;
            char palabra[LARGO];
            float res;
            /* en orden: lo más viejo primero */
            for (i2 = 0; i2 < n / 2; i2++) { int tmp = ids[i2]; ids[i2] = ids[n - 1 - i2]; ids[n - 1 - i2] = tmp; }
            m_texto(m, ids, n, es, sizeof es, resh, sizeof resh);
            snprintf(linea, sizeof linea, "mi sor ra: %s   «recuerdo: %s»", resh, es);
            c_evento(c, rol, EV_DICE, linea);
            res = c_difunde(c, rol, ids, n, &aprend, palabra);
            rec = 1.2f * tanhf(res);
        } else rec = -0.2f;
        break;
    }
    default: {
        int r = nyx_ent(NROLES);
        Mente *o = &c->m[r];
        if (o->nven > 0 && r != rol) {
            const char *ult = o->ven[(o->iven + NVEN - 1) % NVEN];
            const char *dos = strstr(ult, ": ");
            int ids[MAXEST];
            m_recibe(m, dos ? dos + 2 : ult, NYX_ROLES[r].nombre, ids, MAXEST);
            snprintf(t, sizeof t, "escuchó a %s", NYX_ROLES[r].nombre);
            c_evento(c, rol, EV_NOTA, t);
        }
        rec = 0.2f;
        break;
    }
    }
    c_aprende(c, rol, a, rec);
}

/* Pregunta del humano a las 18: deliberación con votos. */
static void consejo_delibera(Consejo *c, const char *pregunta, NyxRespuesta *out) {
    int r, i, j, gan[NROLES];
    float peso[NROLES], C[NROLES], E[NROLES], mejor = -1;
    char etiq[NROLES][LARGO];
    float suma[NROLES];
    int nEtq = 0, ganE = -1;
    memset(out, 0, sizeof *out);
    c->fbValido = 1; c->fbN = 0;
    for (r = 0; r < NROLES; r++) {
        Mente *m = &c->m[r];
        int ids[MAXEST], k = m_recibe(m, pregunta, "humano", ids, MAXEST), cr;
        gan[r] = k > 0 ? m_veredicto(m, ids, k, &C[r], &E[r], &cr) : -1;
        c->fbNest[r] = k;
        for (i = 0; i < k; i++) c->fbEst[r][i] = ids[i];
        c->fbGan[r] = gan[r];
        if (gan[r] < 0) { peso[r] = 0; continue; }
        peso[r] = (0.3f + m_precision(m)) * (0.3f + nyx_max(0, C[r])) * (0.5f + E[r]);
        if (cr >= 0) {
            char t[96];
            snprintf(t, sizeof t, "cristalizó un concepto: «%s»", m->s[cr].et);
            c_evento(c, r, EV_IDEA, t);
        }
    }
    for (r = 0; r < NROLES; r++) {         /* votos por etiqueta */
        if (gan[r] < 0) continue;
        for (j = 0; j < nEtq; j++) if (strcmp(etiq[j], c->m[r].s[gan[r]].et) == 0) break;
        if (j == nEtq) { nyx_copia(etiq[nEtq], c->m[r].s[gan[r]].et, LARGO); suma[nEtq] = 0; nEtq++; }
        suma[j] += peso[r];
    }
    for (j = 0; j < nEtq; j++) if (ganE < 0 || suma[j] > suma[ganE]) ganE = j;
    if (ganE < 0) { snprintf(out->frase, sizeof out->frase, "(silencio: no entendimos nada)"); c->fbValido = 0; return; }
    nyx_copia(out->ganador, etiq[ganE], LARGO);
    out->vocero = -1;
    {
        int acuerdo = 0;
        for (r = 0; r < NROLES; r++) {
            int ok = gan[r] >= 0 && strcmp(c->m[r].s[gan[r]].et, etiq[ganE]) == 0;
            NyxVoto *v = &out->votos[out->nvotos++];
            v->rol = r; v->C = gan[r] >= 0 ? C[r] : 0; v->E = gan[r] >= 0 ? E[r] : 0; v->peso = peso[r]; v->acuerdo = ok;
            nyx_copia(v->atractor, gan[r] >= 0 ? c->m[r].s[gan[r]].et : "-", LARGO);
            if (gan[r] >= 0) {
                c->m[r].intentos++;
                if (ok) c->m[r].aciertos++;
                if (c->m[r].intentos > 200) { c->m[r].intentos = 100; c->m[r].aciertos /= 2; }
            }
            if (ok) {
                acuerdo++;
                c->fbMente[c->fbN++] = r;
                if (peso[r] > mejor) { mejor = peso[r]; out->vocero = r; }
            }
        }
        out->acuerdo = (float)acuerdo / (float)NROLES;
    }
    /* ordena los votos por peso */
    for (i = 0; i < out->nvotos; i++)
        for (j = i + 1; j < out->nvotos; j++)
            if (out->votos[j].peso > out->votos[i].peso) { NyxVoto t = out->votos[i]; out->votos[i] = out->votos[j]; out->votos[j] = t; }
    {
        Mente *v = &c->m[out->vocero];
        int aprend;
        char palabra[LARGO], es[400], resh[400];
        c->fbNfrase = m_frase(v, gan[out->vocero], c->fbFrase, MAXFRASE, c->fbEst[out->vocero], c->fbNest[out->vocero]);
        c->fbVocero = out->vocero;
        m_texto(v, c->fbFrase, c->fbNfrase, es, sizeof es, resh, sizeof resh);
        if (out->acuerdo < 0.25f) {
            snprintf(out->frase, sizeof out->frase, "¿%s?", es);
            snprintf(out->resh, sizeof out->resh, "ye %s?", resh);
        } else {
            snprintf(out->frase, sizeof out->frase, "%s", es);
            snprintf(out->resh, sizeof out->resh, "se ko %s", resh);
        }
        c_difunde(c, out->vocero, c->fbFrase, c->fbNfrase, &aprend, palabra);
    }
}

/* Hablar con una sola mente. */
static void consejo_habla_con(Consejo *c, int rol, const char *texto, char *es, size_t ces, char *resh, size_t cre, float *C) {
    Mente *m = &c->m[rol];
    int ids[MAXEST], k = m_recibe(m, texto, "humano", ids, MAXEST), g, cr;
    float E;
    es[0] = 0; resh[0] = 0; *C = 0;
    c->fbValido = 0;
    if (k == 0) { snprintf(es, ces, "(no entendí)"); return; }
    {   /* si iba a repetir lo que se acaba de decir, piensa otra cosa */
        int intento;
        for (intento = 0; intento < 4; intento++) {
            g = m_veredicto(m, ids, k, C, &E, &cr);
            if (g < 0) { snprintf(es, ces, "(no entendí)"); return; }
            c->fbNfrase = m_frase(m, g, c->fbFrase, MAXFRASE, ids, k);
            m_texto(m, c->fbFrase, c->fbNfrase, es, ces, resh, cre);
            if (!c_es_eco(c, es)) break;
        }
    }
    c_pon_eco(c, es);
    c->fbValido = 1; c->fbN = 1; c->fbMente[0] = rol; c->fbGan[rol] = g; c->fbNest[rol] = k;
    memcpy(c->fbEst[rol], ids, (size_t)k * sizeof ids[0]);
    c->fbVocero = rol;
    m_cansa(m, c->fbFrase, c->fbNfrase);
    if (cr >= 0) {
        char t[96];
        snprintf(t, sizeof t, "cristalizó un concepto: «%s»", m->s[cr].et);
        c_evento(c, rol, EV_IDEA, t);
    }
}

/* Tu opinión sobre la última respuesta reestructura lo aprendido. */
static int consejo_opina(Consejo *c, int bueno) {
    int i, k, x;
    if (!c->fbValido) return 0;
    for (i = 0; i < c->fbN; i++) {
        int r = c->fbMente[i], g = c->fbGan[r];
        Mente *m = &c->m[r];
        if (g < 0) continue;
        for (k = 0; k < c->fbNest[r]; k++) {
            int e = c->fbEst[r][k];
            if (e == g || nyx_vacia(m->s[e].et)) continue;
            if (bueno) { m_refuerza(m, e, g, 0.3f, 0, 0); m_refuerza(m, g, e, 0.2f, 0, 0); }
            else { m_fija(m, e, g, 0.7f, NYX_PI); m_fija(m, g, e, 0.7f, NYX_PI); }
        }
        if (bueno) { m->aciertos++; m->intentos++; m->valor[ACC_HABLAR] += 0.05f; }
        else { m->intentos++; m->valor[ACC_HABLAR] -= 0.05f; }
    }
    if (bueno) {     /* la frase dicha queda como buena secuencia */
        Mente *v = &c->m[c->fbVocero];
        for (x = 0; x + 1 < c->fbNfrase; x++) m_refuerza(v, c->fbFrase[x], c->fbFrase[x + 1], 0.1f, 0.3f, 0.2f);
    }
    c->fbValido = 0;
    return 1;
}

/* ------------------------------------------------------------------ */
/*  Memoria en disco (texto)                                           */
/* ------------------------------------------------------------------ */

static int consejo_guarda(const Consejo *c, const char *ruta) {
    FILE *f = fopen(ruta, "w");
    int r, i, k;
    if (!f) return 0;
    fprintf(f, "NYX 1 %lu\n", c->tick);
    for (r = 0; r < NROLES; r++) {
        const Mente *m = &c->m[r];
        fprintf(f, "MENTE %d %d %d %d %ld %ld %ld %ld %ld\n", r, m->n, m->aciertos, m->intentos,
                m->ciclos, m->dichos, m->ideas, m->cristales, m->reshAprendidas);
        fprintf(f, "VAL");
        for (i = 0; i < NACC; i++) fprintf(f, " %.4f %d", m->valor[i], m->veces[i]);
        fprintf(f, "\nOBJ %d", m->nobj);
        for (i = 0; i < m->nobj; i++) fprintf(f, " %d", m->obj[i]);
        fprintf(f, "\n");
        for (i = 0; i < m->n; i++) {
            const Semion *s = &m->s[i];
            fprintf(f, "S %s %.4f %.4f %.4f %d %d %d %u %u", s->et, s->A, s->fase, s->frec,
                    s->carga, s->fusion, s->sabe, s->usos, s->dialogo);
            for (k = 0; k < DIMF; k++) fprintf(f, " %.4f", s->f[k]);
            fprintf(f, " %d", s->nv);
            for (k = 0; k < s->nv; k++) fprintf(f, " %d %.4f %.4f %.4f", s->v[k].j, s->v[k].w, s->v[k].th, s->v[k].sec);
            fprintf(f, "\n");
        }
    }
    fprintf(f, "FIN\n");
    return fclose(f) == 0;
}

static int consejo_carga(Consejo *c, const char *ruta) {
    static Consejo tmp;    /* se carga aparte: si el archivo está mal, no se pierde nada */
    FILE *f = fopen(ruta, "r");
    int r, i, k, ver;
    char cab[8];
    if (!f) return 0;
    tmp = *c;
    if (fscanf(f, "%7s %d %lu", cab, &ver, &tmp.tick) != 3 || strcmp(cab, "NYX") != 0) { fclose(f); return 0; }
    for (r = 0; r < NROLES; r++) {
        Mente *m = &tmp.m[r];
        int rr, n, nobj;
        if (fscanf(f, " MENTE %d %d %d %d %ld %ld %ld %ld %ld", &rr, &n, &m->aciertos, &m->intentos,
                   &m->ciclos, &m->dichos, &m->ideas, &m->cristales, &m->reshAprendidas) != 9 || rr != r || n < 0 || n > MAXSEM) { fclose(f); return 0; }
        if (fscanf(f, " VAL") != 0) { fclose(f); return 0; }
        for (i = 0; i < NACC; i++) if (fscanf(f, " %f %d", &m->valor[i], &m->veces[i]) != 2) { fclose(f); return 0; }
        if (fscanf(f, " OBJ %d", &nobj) != 1 || nobj < 0 || nobj > 8) { fclose(f); return 0; }
        m->nobj = nobj;
        for (i = 0; i < nobj; i++) { int x; if (fscanf(f, " %d", &x) != 1 || x < 0 || x >= n) { fclose(f); return 0; } m->obj[i] = (short)x; }
        m->n = n;
        for (i = 0; i < n; i++) {
            Semion *s = &m->s[i];
            int carga, fusion, sabe, nv;
            unsigned usos, dialogo;
            memset(s, 0, sizeof *s);
            if (fscanf(f, " S %27s %f %f %f %d %d %d %u %u", s->et, &s->A, &s->fase, &s->frec,
                       &carga, &fusion, &sabe, &usos, &dialogo) != 9) { fclose(f); return 0; }
            s->carga = (signed char)carga; s->fusion = (unsigned char)fusion; s->sabe = (unsigned char)sabe;
            s->usos = (unsigned short)usos; s->dialogo = (unsigned short)dialogo;
            for (k = 0; k < DIMF; k++) if (fscanf(f, " %f", &s->f[k]) != 1) { fclose(f); return 0; }
            if (fscanf(f, " %d", &nv) != 1 || nv < 0 || nv > MAXVEC) { fclose(f); return 0; }
            s->nv = (unsigned char)nv;
            for (k = 0; k < nv; k++) {
                int j;
                if (fscanf(f, " %d %f %f %f", &j, &s->v[k].w, &s->v[k].th, &s->v[k].sec) != 4 || j < 0 || j >= n) { fclose(f); return 0; }
                s->v[k].j = (short)j;
            }
        }
        m->nfoco = 0; m->nepis = 0; m->ncam = 0; m->nven = 0; m->iven = 0; m->nrec = 0;
        m->ultima = -1; m->racha = 0;
        m_hash_rehace(m);
    }
    fclose(f);
    *c = tmp;
    return 1;
}

/* Cuántas palabras sabe decir en Resh una mente. */
static int m_cuenta_resh(const Mente *m) {
    int i, n = 0;
    for (i = 0; i < m->n; i++) if (m->s[i].sabe) n++;
    return n;
}

static int m_mejor_accion(const Mente *m) {
    int a, mejor = 0;
    for (a = 1; a < NACC; a++)
        if (NYX_ROLES[m->rol].gusto[a] + m->valor[a] > NYX_ROLES[m->rol].gusto[mejor] + m->valor[mejor]) mejor = a;
    return mejor;
}

#endif
