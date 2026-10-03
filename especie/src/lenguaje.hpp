#pragma once
// Abla: el idioma con el que nace la especie.
//
// 200 raíces (conceptos) × 20 aspectos = 4000 palabras. Las palabras se
// generan de forma determinista, así que cada ser las conoce desde que
// despierta: no las aprende, nace sabiéndolas. Además, los seres de la
// tribu del lenguaje pueden acuñar palabras compuestas nuevas.
//
// Forma de una palabra: raíz de dos sílabas (CV CV) + sufijo de aspecto
// opcional (una sílaba). Las compuestas terminan en "sh".

#include <algorithm>
#include <cctype>
#include <cstdint>
#include <map>
#include <random>
#include <sstream>
#include <cstdio>
#include <cstdlib>
#include <string>
#include <unordered_map>
#include <vector>

#include "archivos.hpp"

namespace abla {

inline const std::vector<std::string>& raices() {
  static const std::vector<std::string> r = {
      // lenguaje
      "palabra", "frase", "nombre", "verbo", "sonido", "letra", "significado", "idioma", "voz",
      "pregunta", "respuesta", "historia", "signo", "símbolo", "idea", "mensaje", "texto",
      "gramática", "raíz", "sílaba", "silencio", "canto", "diálogo", "metáfora", "traducción",
      "lectura", "escritura", "gesto", "código", "relato", "verso", "eco", "acento", "rima",
      "libro", "página", "tinta", "susurro", "grito", "lengua",
      // datos
      "dato", "número", "cero", "uno", "cantidad", "lista", "tabla", "archivo", "bit", "byte",
      "patrón", "señal", "ruido", "medida", "cuenta", "suma", "resta", "orden", "serie",
      "conjunto", "mapa", "índice", "registro", "flujo", "memoria", "copia", "origen", "destino",
      "valor", "tipo", "tamaño", "tiempo", "fecha", "frecuencia", "promedio", "máximo", "mínimo",
      "muestra", "error", "red",
      // lógica
      "verdad", "mentira", "razón", "causa", "efecto", "regla", "prueba", "si", "entonces", "y",
      "o", "no", "todo", "nada", "algo", "igual", "diferente", "mayor", "menor", "parte",
      "entero", "clase", "ejemplo", "límite", "infinito", "paradoja", "duda", "certeza",
      "hipótesis", "conclusión", "premisa", "contradicción", "implicación", "equivalencia",
      "secuencia", "ciclo", "función", "variable", "constante", "problema",
      // mente
      "yo", "tú", "nosotros", "ellos", "ser", "mente", "cerebro", "pensamiento", "recuerdo",
      "sueño", "deseo", "miedo", "alegría", "curiosidad", "atención", "despertar", "dormir",
      "aprender", "olvidar", "saber", "creer", "buscar", "encontrar", "crear", "dar", "recibir",
      "hablar", "escuchar", "ver", "sentir", "vivir", "nacer", "cambiar", "crecer", "unir",
      "separar", "compartir", "preguntar", "responder", "imaginar",
      // mundo
      "luz", "sombra", "agua", "fuego", "tierra", "aire", "cielo", "sol", "luna", "estrella",
      "árbol", "semilla", "río", "mar", "montaña", "camino", "casa", "puerta", "ventana",
      "piedra", "hierro", "vidrio", "día", "noche", "inicio", "fin", "arriba", "abajo", "dentro",
      "fuera", "cerca", "lejos", "rápido", "lento", "grande", "pequeño", "nuevo", "viejo",
      "caliente", "frío",
  };
  return r;
}

inline const std::vector<std::string>& aspectos() {
  static const std::vector<std::string> a = {
      "",          "muchos",    "negado", "pregunta", "pasado",   "futuro",   "que causa",
      "resultado", "muy",       "poco",   "comienzo", "final",    "trozo",    "totalidad",
      "como",      "contrario", "quien",  "lugar",    "acto",     "cualidad",
  };
  return a;
}

// Aspectos con nombre, para usarlos en el código.
enum Aspecto { BASE_ASP = 0, MUCHOS = 1, NEGADO = 2, PREGUNTA = 3, QUIEN = 16 };

// Relaciones entre conceptos. Cada una se dice con una raíz de Abla.
enum Rel : uint8_t { ES = 0, PARTE = 1, CAUSA = 2, IGUAL = 3, OPUESTO = 4, NREL = 5 };
inline const char* relNombre(int r) {
  static const char* n[] = {"es", "parte", "causa", "igual", "opuesto"};
  return n[r];
}
inline const char* relGlosa(int r) {
  static const char* n[] = {"es", "es parte de", "causa", "se parece a", "es lo opuesto de"};
  return n[r];
}
inline const char* relRaiz(int r) {
  static const char* n[] = {"ser", "parte", "causa", "igual", "diferente"};
  return n[r];
}
inline int relDeNombre(const std::string& s) {
  for (int r = 0; r < NREL; r++)
    if (s == relNombre(r)) return r;
  return -1;
}

inline std::string minusculas(std::string s) {
  for (auto& c : s) c = static_cast<char>(std::tolower(static_cast<unsigned char>(c)));
  return s;
}

// Barajado determinista: mt19937 da la misma secuencia en todas partes, pero
// std::shuffle no (libstdc++ y libc++ barajan distinto). Con este, Abla es el
// mismo idioma en Linux, Termux, iSH y Code App.
template <class T>
void barajar(std::vector<T>& v, std::mt19937& g) {
  for (size_t i = v.size(); i > 1; i--) std::swap(v[i - 1], v[g() % i]);
}

class Idioma {
 public:
  static constexpr int RAICES = 200, ASPECTOS = 20, BASE = RAICES * ASPECTOS;  // 4000
  static constexpr int MAX_PALABRAS = 60000;

  Idioma() {
    if (static_cast<int>(raices().size()) != RAICES || static_cast<int>(aspectos().size()) != ASPECTOS) {
      std::fprintf(stderr, "Abla: el léxico base debe tener 200 raíces y 20 aspectos\n");
      std::abort();
    }
    std::mt19937 g(0xAB1A);  // misma semilla siempre: todos nacen con el mismo idioma
    const std::string C = "ktsnmlrvzp", V = "aeiou";
    std::vector<std::string> silabas;
    for (char c : C)
      for (char v : V) silabas.push_back({c, v});
    std::vector<std::string> pares;
    for (auto& a : silabas)
      for (auto& b : silabas) pares.push_back(a + b);
    barajar(pares, g);
    std::vector<std::string> sufijos;
    for (char c : std::string("dgbfjh"))
      for (char v : V) sufijos.push_back({c, v});
    barajar(sufijos, g);

    for (int r = 0; r < RAICES; r++) {
      porRaiz_[raices()[r]] = r;
      for (int a = 0; a < ASPECTOS; a++) {
        std::string f = pares[r] + (a ? sufijos[a - 1] : "");
        porForma_[f] = static_cast<int>(formas_.size());
        formas_.push_back(f);
      }
    }
  }

  static int id(int raiz, int aspecto = 0) { return raiz * ASPECTOS + aspecto; }
  static bool esBase(int id) { return id >= 0 && id < BASE; }
  static int raizDe(int id) { return id / ASPECTOS; }
  static int aspectoDe(int id) { return id % ASPECTOS; }

  int tamano() const { return static_cast<int>(formas_.size()); }
  int compuestas() const { return static_cast<int>(compuestos_.size()); }
  bool valida(int id) const { return id >= 0 && id < tamano(); }
  const std::string& forma(int id) const { return formas_.at(id); }

  std::string glosa(int id) const {
    if (!valida(id)) return "?";
    if (id >= BASE) {
      const auto& c = compuestos_[id - BASE];
      return glosa(c.first) + "+" + glosa(c.second);
    }
    std::string g = raices()[raizDe(id)];
    if (int a = aspectoDe(id)) g += "·" + aspectos()[a];
    return g;
  }

  std::pair<int, int> partes(int id) const { return compuestos_.at(id - BASE); }

  int deForma(const std::string& f) const {
    auto it = porForma_.find(minusculas(f));
    return it == porForma_.end() ? -1 : it->second;
  }
  // Palabra en español (raíz) → palabra base en Abla.
  int deEspanol(const std::string& w) const {
    auto it = porRaiz_.find(minusculas(w));
    return it == porRaiz_.end() ? -1 : id(it->second);
  }
  int relacion(int r) const { return deEspanol(relRaiz(r)); }
  int relDePalabra(int palabra) const {
    for (int r = 0; r < NREL; r++)
      if (relacion(r) == palabra) return r;
    return -1;
  }

  // Crea (o devuelve) la palabra compuesta para "a+b".
  int acunar(int a, int b) {
    auto clave = std::make_pair(a, b);
    if (auto it = porPar_.find(clave); it != porPar_.end()) return it->second;
    if (tamano() >= MAX_PALABRAS) return -1;
    std::string f = forma(a).substr(0, 2) + forma(b).substr(0, 2) + "sh";
    if (porForma_.count(f)) f = forma(a) + forma(b) + "sh";
    while (porForma_.count(f)) f += "a";
    int nuevo = tamano();
    formas_.push_back(f);
    porForma_[f] = nuevo;
    compuestos_.push_back(clave);
    porPar_[clave] = nuevo;
    return nuevo;
  }
  int compuestaDe(int a, int b) const {
    auto it = porPar_.find({a, b});
    return it == porPar_.end() ? -1 : it->second;
  }

  std::string frase(const std::vector<int>& ids) const {
    std::string s;
    for (int i : ids) s += (s.empty() ? "" : " ") + (valida(i) ? forma(i) : "?");
    return s;
  }
  std::string glosaFrase(const std::vector<int>& ids) const {
    std::string s;
    for (int i : ids) s += (s.empty() ? "" : " ") + glosa(i);
    return s;
  }

  // Lee texto: acepta palabras de Abla o raíces en español.
  std::vector<int> leer(const std::string& texto, std::vector<std::string>* desconocidas = nullptr) const {
    std::vector<int> ids;
    std::istringstream in(texto);
    std::string w;
    while (in >> w) {
      int i = deForma(w);
      if (i < 0) i = deEspanol(w);
      if (i >= 0) ids.push_back(i);
      else if (desconocidas) desconocidas->push_back(w);
    }
    return ids;
  }

  void exportarDiccionario(const std::string& archivo) const {
    std::ostringstream out;
    out << "# Diccionario de Abla: " << tamano() << " palabras (" << BASE << " innatas + "
        << compuestas() << " creadas por la especie)\n# id\tpalabra\tsignificado\n";
    for (int i = 0; i < tamano(); i++) out << i << '\t' << forma(i) << '\t' << glosa(i) << '\n';
    archivos::escribir(archivo, out.str());
  }
  void guardarCompuestas(const std::string& archivo) const {
    std::ostringstream out;
    for (int i = 0; i < compuestas(); i++)
      out << BASE + i << ' ' << formas_[BASE + i] << ' ' << compuestos_[i].first << ' '
          << compuestos_[i].second << '\n';
    archivos::escribir(archivo, out.str());
  }
  void cargarCompuestas(const std::string& archivo) {
    std::string datos;
    archivos::leer(archivo, datos);
    std::istringstream in(datos);
    int id, a, b;
    std::string f;
    while (in >> id >> f >> a >> b) {
      if (id != tamano() || !valida(a) || !valida(b)) break;  // orden roto: se ignora el resto
      formas_.push_back(f);
      porForma_[f] = id;
      compuestos_.push_back({a, b});
      porPar_[{a, b}] = id;
    }
  }

 private:
  std::vector<std::string> formas_;
  std::unordered_map<std::string, int> porForma_, porRaiz_;
  std::vector<std::pair<int, int>> compuestos_;
  std::map<std::pair<int, int>, int> porPar_;
};

}  // namespace abla
