/* base.c — azar, textos y cómo es cada una de las 18 */
#include "nyx.h"

/* ------------------------------------------------------------------ */
/*  Azar (xorshift: igual en todas las plataformas)                    */
/* ------------------------------------------------------------------ */

static unsigned long long nyx_azar_estado = 88172645463325252ULL;

void nyx_semilla(unsigned long long s) {
    nyx_azar_estado = s ? s : 88172645463325252ULL;
}

unsigned nyx_u32(void) {
    unsigned long long x = nyx_azar_estado;
    x ^= x << 13;
    x ^= x >> 7;
    x ^= x << 17;
    nyx_azar_estado = x;
    return (unsigned)(x >> 32);
}


unsigned nyx_fnv(const char *s) {
    unsigned h = 2166136261u;
    while (*s) { h ^= (unsigned char)*s++; h *= 16777619u; }
    return h;
}

void nyx_copia(char *dst, const char *src, size_t cap) {
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

const char *NYX_ACC[NACC] = { "hablar", "preguntar", "imaginar", "soñar", "recordar", "escuchar" };


const RolInfo NYX_ROLES[NROLES] = {
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

int nyx_rol_de(const char *nombre) {
    int r;
    for (r = 0; r < NROLES; r++)
        if (strcmp(NYX_ROLES[r].nombre, nombre) == 0) return r;
    /* también vale el comienzo: "crea" -> creativo */
    for (r = 0; r < NROLES; r++)
        if (strlen(nombre) >= 3 && strncmp(NYX_ROLES[r].nombre, nombre, strlen(nombre)) == 0) return r;
    return -1;
}

/* Lo que leen al nacer: la infancia de las 18 (si no hay memoria guardada). */
const char *NYX_CORPUS[] = {
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
const int nyx_ncorpus = (int)(sizeof NYX_CORPUS / sizeof NYX_CORPUS[0]);

/* Palabras vacías: sirven para la gramática, nunca para ganar un veredicto. */
static const char *NYX_VACIAS[] = {
    "el", "la", "los", "las", "un", "una", "unos", "unas", "de", "del", "al", "a", "y", "o", "u", "e",
    "que", "en", "por", "con", "para", "se", "su", "sus", "lo", "le", "les", "es", "son", "no", "mi",
    "tu", "me", "te", "nos", "ya", "más", "mas", "muy", "pero", "si", "sí", "como", "cuando", "hay",
    "este", "esta", "esto", "ese", "esa", "eso", "fue", "ser", "está", "están", "qué", "cómo", "yo",
    "tú", "él", "ella", "todo", "toda", "sin", "sobre", "entre", "hasta", "desde", "también", "ni",
};

int nyx_vacia(const char *w) {
    size_t i;
    for (i = 0; i < sizeof NYX_VACIAS / sizeof NYX_VACIAS[0]; i++)
        if (strcmp(NYX_VACIAS[i], w) == 0) return 1;
    return 0;
}

int nyx_conector(const char *w) {
    static const char *c[] = { "y", "o", "u", "e", "en", "por", "con", "de", "del", "que", "a", "al", "para", "ni", "pero", "sin", "como", "se", "es", "son" };
    size_t i;
    for (i = 0; i < sizeof c / sizeof c[0]; i++) if (strcmp(c[i], w) == 0) return 1;
    return 0;
}
