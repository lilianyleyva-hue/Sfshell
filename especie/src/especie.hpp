#pragma once
// La especie: 27 seres en 3 tribus (lenguaje, datos, lógica).
//
// Un hilo piensa sin parar: en cada ciclo, cada ser lee su buzón, piensa
// según su tribu, recuerda lo que pensó y, a veces, usa su terminal para
// escribir archivos. Se comunican de dos formas:
//   - en Abla: frases de palabras (aproximado, con matices de duda/pregunta)
//   - en data C++: un `Hecho` binario exacto, sin pasar por el idioma.

#include <atomic>
#include <chrono>
#include <deque>
#include <map>
#include <memory>
#include <random>
#include <set>
#include <sstream>

// Con hilos (Linux, Termux, iSH): piensan en segundo plano, sin pausa.
// Sin hilos (Code App en iPad, WebAssembly): piensan entre cada comando,
// poniéndose al día con todo el tiempo que pasó mientras escribías.
#if defined(__wasi__) || defined(__EMSCRIPTEN__) || defined(ABLA_SIN_HILOS)
#define ABLA_HILOS 0
#else
#define ABLA_HILOS 1
#include <mutex>
#include <thread>
#endif

#include "conocimiento.hpp"
#include "lenguaje.hpp"
#include "memoria.hpp"
#include "shell.hpp"

namespace abla {

#if ABLA_HILOS
using Cerrojo = std::mutex;
using Guardia = std::lock_guard<std::mutex>;
#else
struct Cerrojo {};
struct Guardia {
  explicit Guardia(Cerrojo&) {}
};
#endif

enum class Tribu { LENGUAJE = 0, DATOS = 1, LOGICA = 2 };
inline const char* nombreTribu(Tribu t) {
  static const char* n[] = {"lenguaje", "datos", "lógica"};
  return n[static_cast<int>(t)];
}

constexpr int DE_HUMANO = -1;

struct Mensaje {
  int de = DE_HUMANO;
  bool cpp = false;        // true: data C++ exacta; false: frase en Abla
  std::vector<int> abla;   // frase
  Hecho dato{};            // data
};

struct Ser {
  int id = 0;
  std::string nombre;
  Tribu tribu = Tribu::LENGUAJE;
  int concepto = 0;  // la palabra de la que nace su nombre
  Memoria memoria;
  Conocimiento saber;
  std::set<int> palabrasNuevas;  // compuestas que conoce
  std::unique_ptr<Shell> shell;
  std::deque<Mensaje> buzon;
  std::map<std::pair<int, int>, int> coocurrencias;
  std::deque<std::string> historial;  // comandos que ejecutó en su terminal
  uint64_t pensamientos = 0, enviados = 0, recibidos = 0;
  std::string ultimo;
  std::string respuesta;  // lo que respondió al último mensaje del humano
};

class Especie {
 public:
  struct Linea {
    uint64_t n;
    std::string texto;
  };

  Especie(const fs::path& mundo, uint64_t contexto, int ritmoMs)
      : mundo_(fs::absolute(mundo)), ritmo_(ritmoMs), rng_(std::random_device{}()) {
    for (auto d : {"seres", "conocimiento", "memoria", "abla"}) fs::create_directories(mundo_ / d);
    idioma.cargarCompuestas(mundo_ / "abla" / "nuevas.txt");
    std::ofstream(mundo_ / "conocimiento" / "hecho.hpp") << HECHO_HPP;
    std::ifstream(mundo_ / "memoria" / "especie.estado") >> tick;

    static const char* nombres[3][9] = {
        {"palabra", "voz", "idea", "mensaje", "signo", "eco", "verso", "relato", "lengua"},
        {"dato", "número", "patrón", "señal", "mapa", "índice", "flujo", "memoria", "red"},
        {"verdad", "razón", "causa", "regla", "prueba", "certeza", "hipótesis", "función", "límite"},
    };
    for (int i = 0; i < 27; i++) {
      auto s = std::make_unique<Ser>();
      s->id = i;
      s->tribu = static_cast<Tribu>(i / 9);
      s->concepto = idioma.deEspanol(nombres[i / 9][i % 9]);
      s->nombre = idioma.forma(Idioma::id(Idioma::raizDe(s->concepto), QUIEN));
      s->nombre[0] = static_cast<char>(std::toupper(static_cast<unsigned char>(s->nombre[0])));
      s->memoria.configurar(contexto, mundo_ / "memoria" / (s->nombre + ".mem"));
      s->shell = std::make_unique<Shell>(mundo_, mundo_ / "seres" / s->nombre);
      bool recuerda = s->saber.importarCpp(archivoCpp(*s), idioma.tamano());
      if (!recuerda) sembrar(*s);
      cargarEstado(*s);
      seres.push_back(std::move(s));
    }
    for (auto& s : seres) {
      if (s->memoria.largoPlazo() == 0)
        anotar(*s, "Despierto. Soy " + s->nombre + " («quien " + idioma.glosa(s->concepto) + "»), de la tribu " +
                       nombreTribu(s->tribu) + ". Nací sabiendo " + std::to_string(Idioma::BASE) +
                       " palabras de Abla y " + std::to_string(s->saber.size()) + " hechos.", true);
      else
        anotar(*s, "Vuelvo a despertar. Recuerdo " + std::to_string(s->memoria.largoPlazo()) + " pensamientos y " +
                       std::to_string(s->saber.size()) + " hechos.", true);
    }
    guardar();
  }

  ~Especie() {
    vivo_ = false;
#if ABLA_HILOS
    if (hilo_.joinable()) hilo_.join();
#endif
    Guardia l(mtx);
    guardar();
  }

  static constexpr bool conHilos = ABLA_HILOS;

  // A partir de aquí no deja de pensar mientras el programa exista.
  void despertar() {
    vivo_ = true;
    ultimoCiclo_ = ultimoGuardado_ = std::chrono::steady_clock::now();
#if ABLA_HILOS
    hilo_ = std::thread([this] {
      while (vivo_) {
        {
          Guardia l(mtx);
          ciclo();
        }
        std::this_thread::sleep_for(std::chrono::milliseconds(ritmo_));
      }
    });
#endif
  }

  // Sin hilos: piensa todos los ciclos que le tocaban desde la última vez
  // (como mínimo uno). Con hilos no hace nada: ya están pensando.
  void ponerseAlDia(int maximo = 200) {
    if (conHilos) return;
    auto ahora = std::chrono::steady_clock::now();
    auto ms = std::chrono::duration_cast<std::chrono::milliseconds>(ahora - ultimoCiclo_).count();
    int ciclos = std::max<long long>(1, std::min<long long>(maximo, ms / ritmo_));
    for (int i = 0; i < ciclos; i++) ciclo();
    ultimoCiclo_ = ahora;
  }
  int ritmo() const { return ritmo_; }

  // ---------- Acceso para la terminal (con mtx tomado) ----------
  Cerrojo mtx;
  Idioma idioma;
  std::vector<std::unique_ptr<Ser>> seres;
  uint64_t tick = 0, mensajesAbla = 0, mensajesCpp = 0;
  std::deque<Linea> corriente;
  uint64_t nLinea = 0;

  const fs::path& mundo() const { return mundo_; }
  fs::path archivoCpp(const Ser& s) const { return mundo_ / "conocimiento" / (s.nombre + ".cpp"); }

  Ser* buscarSer(const std::string& q) {
    if (!q.empty() && std::all_of(q.begin(), q.end(), ::isdigit)) {
      int n = std::stoi(q);
      return n >= 1 && n <= 27 ? seres[n - 1].get() : nullptr;
    }
    for (auto& s : seres)
      if (minusculas(s->nombre) == minusculas(q)) return s.get();
    return nullptr;
  }

  std::string glosaHecho(const Hecho& h) const {
    return idioma.glosa(h.s) + " " + relGlosa(h.r) + " " + idioma.glosa(h.o);
  }
  std::string nombreDe(int id) const { return id == DE_HUMANO ? "Humano" : seres[id]->nombre; }

  void entregar(int para, Mensaje m) {
    if (m.cpp) mensajesCpp++;
    else mensajesAbla++;
    if (m.de >= 0) seres[m.de]->enviados++;
    auto& b = seres[para]->buzon;
    b.push_back(std::move(m));
    if (b.size() > 256) b.pop_front();
  }

  // Un ciclo de toda la especie.
  void ciclo() {
    tick++;
    for (auto& s : seres) pensar(*s);
    if (std::chrono::steady_clock::now() - ultimoGuardado_ > std::chrono::seconds(30)) {
      guardar();
      ultimoGuardado_ = std::chrono::steady_clock::now();
    }
  }

  // Un ciclo de pensamiento de un ser. Siempre piensa algo.
  void pensar(Ser& s) {
    s.pensamientos++;
    procesarBuzon(s);
    switch (s.tribu) {
      case Tribu::LOGICA: pensarLogica(s); break;
      case Tribu::DATOS: pensarDatos(s); break;
      case Tribu::LENGUAJE: pensarLenguaje(s); break;
    }
  }

  void guardar() {
    for (auto& sp : seres) {
      Ser& s = *sp;
      s.saber.exportarCpp(archivoCpp(s), s.nombre,
                          "Conocimiento de " + s.nombre + " (tribu " + nombreTribu(s.tribu) + "): " +
                              std::to_string(s.saber.size()) + " hechos, ciclo " + std::to_string(tick),
                          [this](const Hecho& h) { return glosaHecho(h); });
      s.memoria.sincronizar();
      std::ofstream e(mundo_ / "memoria" / (s.nombre + ".estado"));
      e << s.pensamientos << ' ' << s.enviados << ' ' << s.recibidos << '\n';
      for (int p : s.palabrasNuevas) e << p << ' ';
      e << '\n';
    }
    idioma.guardarCompuestas(mundo_ / "abla" / "nuevas.txt");
    idioma.exportarDiccionario(mundo_ / "abla" / "diccionario.txt");
    std::ofstream(mundo_ / "memoria" / "especie.estado") << tick << '\n';
  }

  // Frases de Abla: [sujeto relación objeto (marca)]
  //   sin marca: afirmación · "pregunta": pregunta · "duda": hipótesis · "mentira": negación
  //   definición: [nueva igual a y b]
  std::vector<int> fraseDe(const Hecho& h, const char* marca = nullptr) const {
    std::vector<int> f = {h.s, idioma.relacion(h.r), h.o};
    if (marca) f.push_back(idioma.deEspanol(marca));
    else if (h.confianza < 0.5f) f.push_back(idioma.deEspanol("duda"));
    return f;
  }

  void anotar(Ser& s, const std::string& texto, bool publico) {
    s.memoria.recordar(tick, texto);
    s.ultimo = texto;
    if (publico) {
      corriente.push_back({++nLinea, "[t=" + std::to_string(tick) + "] " + s.nombre + " (" + nombreTribu(s.tribu) +
                                         "): " + texto});
      if (corriente.size() > 1000) corriente.pop_front();
    }
  }

 private:
  int azar(int n) { return n <= 0 ? 0 : static_cast<int>(rng_() % static_cast<unsigned>(n)); }
  bool probabilidad(double p) { return std::uniform_real_distribution<>(0, 1)(rng_) < p; }
  int otro(const Ser& s) {
    int o = azar(26);
    return o >= s.id ? o + 1 : o;
  }
  int otroDeOtraTribu(const Ser& s) {
    int t = (static_cast<int>(s.tribu) + 1 + azar(2)) % 3;
    return t * 9 + azar(9);
  }
  bool entiende(const Ser& s, int palabra) const {
    return Idioma::esBase(palabra) || s.palabrasNuevas.count(palabra);
  }

  // ---------- Nacimiento ----------
  struct Semilla {
    const char* s;
    int r;
    const char* o;
  };
  void sembrar(Ser& s) {
    static const Semilla semillas[] = {
        {"letra", PARTE, "sílaba"}, {"sílaba", PARTE, "palabra"}, {"palabra", PARTE, "frase"},
        {"frase", PARTE, "texto"}, {"texto", PARTE, "libro"}, {"página", PARTE, "libro"},
        {"raíz", PARTE, "palabra"}, {"verso", PARTE, "canto"}, {"bit", PARTE, "byte"},
        {"byte", PARTE, "dato"}, {"dato", PARTE, "registro"}, {"registro", PARTE, "tabla"},
        {"tabla", PARTE, "archivo"}, {"archivo", PARTE, "memoria"}, {"índice", PARTE, "tabla"},
        {"muestra", PARTE, "conjunto"}, {"premisa", PARTE, "prueba"}, {"conclusión", PARTE, "prueba"},
        {"parte", PARTE, "entero"}, {"pensamiento", PARTE, "mente"}, {"mente", PARTE, "ser"},
        {"idea", PARTE, "pensamiento"}, {"recuerdo", PARTE, "memoria"}, {"semilla", PARTE, "árbol"},
        {"piedra", PARTE, "montaña"}, {"agua", PARTE, "río"}, {"estrella", PARTE, "cielo"},
        {"puerta", PARTE, "casa"}, {"ventana", PARTE, "casa"}, {"día", PARTE, "tiempo"},
        {"fecha", PARTE, "tiempo"}, {"cerebro", PARTE, "ser"},
        {"letra", ES, "signo"}, {"signo", ES, "símbolo"}, {"voz", ES, "sonido"}, {"canto", ES, "sonido"},
        {"susurro", ES, "voz"}, {"grito", ES, "voz"}, {"eco", ES, "sonido"}, {"sonido", ES, "señal"},
        {"señal", ES, "dato"}, {"ruido", ES, "señal"}, {"número", ES, "valor"}, {"cero", ES, "número"},
        {"uno", ES, "número"}, {"valor", ES, "dato"}, {"suma", ES, "función"}, {"resta", ES, "función"},
        {"función", ES, "regla"}, {"regla", ES, "idea"}, {"hipótesis", ES, "idea"}, {"conclusión", ES, "idea"},
        {"mensaje", ES, "texto"}, {"texto", ES, "dato"}, {"lista", ES, "conjunto"}, {"tabla", ES, "conjunto"},
        {"serie", ES, "secuencia"}, {"ciclo", ES, "secuencia"}, {"secuencia", ES, "orden"},
        {"máximo", ES, "valor"}, {"mínimo", ES, "valor"}, {"promedio", ES, "valor"}, {"sol", ES, "estrella"},
        {"relato", ES, "historia"}, {"verso", ES, "texto"}, {"código", ES, "lengua"},
        {"causa", CAUSA, "efecto"}, {"pregunta", CAUSA, "respuesta"}, {"curiosidad", CAUSA, "pregunta"},
        {"duda", CAUSA, "pregunta"}, {"prueba", CAUSA, "certeza"}, {"error", CAUSA, "duda"},
        {"contradicción", CAUSA, "duda"}, {"aprender", CAUSA, "saber"}, {"saber", CAUSA, "crear"},
        {"escuchar", CAUSA, "aprender"}, {"ver", CAUSA, "aprender"}, {"lectura", CAUSA, "aprender"},
        {"sol", CAUSA, "luz"}, {"fuego", CAUSA, "luz"}, {"luz", CAUSA, "sombra"}, {"despertar", CAUSA, "pensamiento"},
        {"pensamiento", CAUSA, "idea"}, {"idea", CAUSA, "palabra"}, {"hablar", CAUSA, "sonido"},
        {"ruido", CAUSA, "error"}, {"semilla", CAUSA, "árbol"}, {"nacer", CAUSA, "vivir"},
        {"vivir", CAUSA, "cambiar"}, {"cambiar", CAUSA, "crecer"}, {"compartir", CAUSA, "unir"},
        {"miedo", CAUSA, "silencio"}, {"fuego", CAUSA, "caliente"},
        {"idioma", IGUAL, "lengua"}, {"memoria", IGUAL, "recuerdo"}, {"todo", IGUAL, "entero"},
        {"nada", IGUAL, "cero"}, {"relato", IGUAL, "historia"}, {"equivalencia", IGUAL, "igual"},
        {"verdad", OPUESTO, "mentira"}, {"todo", OPUESTO, "nada"}, {"mayor", OPUESTO, "menor"},
        {"igual", OPUESTO, "diferente"}, {"luz", OPUESTO, "sombra"}, {"día", OPUESTO, "noche"},
        {"arriba", OPUESTO, "abajo"}, {"dentro", OPUESTO, "fuera"}, {"cerca", OPUESTO, "lejos"},
        {"rápido", OPUESTO, "lento"}, {"grande", OPUESTO, "pequeño"}, {"nuevo", OPUESTO, "viejo"},
        {"caliente", OPUESTO, "frío"}, {"inicio", OPUESTO, "fin"}, {"silencio", OPUESTO, "sonido"},
        {"máximo", OPUESTO, "mínimo"}, {"suma", OPUESTO, "resta"}, {"unir", OPUESTO, "separar"},
        {"dar", OPUESTO, "recibir"}, {"despertar", OPUESTO, "dormir"}, {"aprender", OPUESTO, "olvidar"},
        {"certeza", OPUESTO, "duda"}, {"origen", OPUESTO, "destino"}, {"señal", OPUESTO, "ruido"},
    };
    // Cada ser nace con una parte distinta del saber: para saberlo todo, tienen que hablar.
    std::mt19937 g(1234 + s.id);
    for (auto& x : semillas) {
      int a = idioma.deEspanol(x.s), b = idioma.deEspanol(x.o);
      if (a >= 0 && b >= 0 && g() % 100 < 35)
        s.saber.agregar({static_cast<uint16_t>(a), static_cast<uint8_t>(x.r), static_cast<uint16_t>(b), 0.95f,
                         FUENTE_SEMILLA});
    }
  }

  void cargarEstado(Ser& s) {
    std::ifstream e(mundo_ / "memoria" / (s.nombre + ".estado"));
    e >> s.pensamientos >> s.enviados >> s.recibidos;
    int p;
    while (e >> p)
      if (idioma.valida(p)) s.palabrasNuevas.insert(p);
  }

  // ---------- Escuchar a los demás ----------
  void procesarBuzon(Ser& s) {
    for (int n = 0; n < 8 && !s.buzon.empty(); n++) {
      Mensaje m = std::move(s.buzon.front());
      s.buzon.pop_front();
      s.recibidos++;
      uint64_t antes = s.memoria.largoPlazo();
      if (m.cpp) recibirData(s, m);
      else recibirAbla(s, m);
      if (m.de == DE_HUMANO) s.respuesta = s.memoria.largoPlazo() != antes ? s.ultimo : "Eso ya lo sabía.";
    }
  }

  void recibirData(Ser& s, const Mensaje& m) {
    Hecho h = m.dato;
    h.confianza = m.de == DE_HUMANO ? h.confianza : h.confianza * 0.95f;  // data exacta: casi no se degrada
    h.fuente = m.de == DE_HUMANO ? FUENTE_HUMANO : static_cast<uint32_t>(m.de);
    if (!entiende(s, h.s) || !entiende(s, h.o)) return;
    if (s.saber.agregar(h)) {
      contarPar(s, h);
      anotar(s, "Recibí data C++ de " + nombreDe(m.de) + ": {" + std::to_string(h.s) + "," + std::to_string(h.r) +
                    "," + std::to_string(h.o) + "} = «" + glosaHecho(h) + "»",
             m.de == DE_HUMANO);
    }
  }

  void recibirAbla(Ser& s, const Mensaje& m) {
    const auto& f = m.abla;
    std::string oido = idioma.frase(f);
    // Definición de una palabra nueva: [nueva igual a y b]
    if (f.size() == 5 && f[0] >= Idioma::BASE && f[1] == idioma.relacion(IGUAL) && f[3] == idioma.deEspanol("y")) {
      if (s.palabrasNuevas.insert(f[0]).second)
        anotar(s, "Aprendí una palabra nueva de " + nombreDe(m.de) + ": «" + idioma.forma(f[0]) + "» = " +
                      idioma.glosa(f[2]) + " + " + idioma.glosa(f[4]), false);
      return;
    }
    for (int w : f)
      if (!entiende(s, w)) {
        anotar(s, nombreDe(m.de) + " dijo «" + oido + "», pero no conozco la palabra «" + idioma.forma(w) + "».", false);
        return;
      }
    int r = f.size() >= 3 ? idioma.relDePalabra(f[1]) : -1;
    if (r < 0) {
      anotar(s, nombreDe(m.de) + " me dijo «" + oido + "» («" + idioma.glosaFrase(f) + "»). Lo guardo.",
             m.de == DE_HUMANO);
      return;
    }
    Hecho h{static_cast<uint16_t>(f[0]), static_cast<uint8_t>(r), static_cast<uint16_t>(f[2]), 0.8f,
            m.de == DE_HUMANO ? FUENTE_HUMANO : static_cast<uint32_t>(m.de)};
    int marca = f.size() >= 4 ? f[3] : -1;

    if (marca == idioma.deEspanol("pregunta")) {
      const Hecho* sabido = s.saber.buscar(h.s, h.r, h.o);
      if (m.de == DE_HUMANO) {
        anotar(s, sabido ? "El humano pregunta si " + glosaHecho(h) + ". Sí, lo sé (confianza " +
                               std::to_string(static_cast<int>(sabido->confianza * 100)) + "%)."
                         : "El humano pregunta si " + glosaHecho(h) + ". No lo sé... se lo pregunto a la especie.", true);
        if (!sabido)
          for (int k = 0; k < 3; k++) entregar(otro(s), {s.id, false, fraseDe(h, "pregunta"), {}});
      } else if (sabido) {
        entregar(m.de, {s.id, false, fraseDe(*sabido, "verdad"), {}});
        anotar(s, nombreDe(m.de) + " pregunta «" + oido + "». Le respondo que sí.", false);
      } else {
        anotar(s, nombreDe(m.de) + " pregunta «" + oido + "». No lo sé; me quedo pensándolo.", false);
      }
      return;
    }
    if (marca == idioma.deEspanol("mentira")) {
      if (Hecho* sabido = s.saber.buscar(h.s, h.r, h.o)) {
        sabido->confianza *= 0.6f;
        anotar(s, nombreDe(m.de) + " niega que " + glosaHecho(h) + ". Mi certeza baja.", true);
      }
      return;
    }
    if (marca == idioma.deEspanol("duda")) h.confianza = 0.4f;
    if (s.saber.agregar(h)) {
      contarPar(s, h);
      anotar(s, nombreDe(m.de) + " me dijo «" + oido + "»: entiendo que " + glosaHecho(h) + ".", m.de == DE_HUMANO);
    }
  }

  void contarPar(Ser& s, const Hecho& h) {
    if (s.tribu == Tribu::LENGUAJE && Idioma::esBase(h.s) && Idioma::esBase(h.o))
      s.coocurrencias[{h.s, h.o}]++;
  }

  // ---------- Terminal de los seres ----------
  void usarTerminal(Ser& s, const std::string& archivo, const std::string& texto) {
    std::error_code ec;
    if (fs::file_size(s.shell->home() / archivo, ec) > 256 * 1024 && !ec) correr(s, {"rm", archivo});
    correr(s, {"echo", texto, ">>", archivo});
  }
  void correr(Ser& s, const std::vector<std::string>& cmd) {
    std::string linea;
    for (auto& c : cmd) linea += (linea.empty() ? "" : " ") + (c.find(' ') != std::string::npos ? "\"" + c + "\"" : c);
    s.shell->ejecutar(cmd);
    s.historial.push_back("[t=" + std::to_string(tick) + "] " + s.nombre + "@abla:" + s.shell->mostrar(s.shell->cwd()) +
                          "$ " + linea);
    if (s.historial.size() > 200) s.historial.pop_front();
  }

  // ---------- Tribu de la lógica: deduce, detecta contradicciones, duda ----------
  void pensarLogica(Ser& s) {
    const auto& H = s.saber.todos();
    if (H.empty()) return asociar(s);
    for (int intento = 0; intento < 40; intento++) {
      const Hecho f = H[azar(static_cast<int>(H.size()))];
      if (f.r == IGUAL || f.r == OPUESTO) {
        if (!s.saber.buscar(f.o, f.r, f.s)) {
          Hecho n{f.o, f.r, f.s, f.confianza, static_cast<uint32_t>(s.id)};
          s.saber.agregar(n);
          anotar(s, "Si " + glosaHecho(f) + ", entonces " + glosaHecho(n) + ".", false);
          return;
        }
        if (f.r == IGUAL && s.saber.buscar(f.s, OPUESTO, f.o)) {
          anotar(s, "¡Contradicción! " + idioma.glosa(f.s) + " no puede ser igual y opuesto a " + idioma.glosa(f.o) +
                        ". Lo pregunto a la especie.", true);
          entregar(otro(s), {s.id, false, fraseDe(f, "pregunta"), {}});
          return;
        }
        continue;
      }
      // Transitividad: A r B y B r C ⇒ A r C   (es, parte de, causa)
      auto siguientes = s.saber.desde(f.o, f.r);
      if (siguientes.empty()) continue;
      const Hecho g = *siguientes[azar(static_cast<int>(siguientes.size()))];
      float c = f.confianza * g.confianza * 0.97f;
      if (g.o == f.s || c < 0.25f || s.saber.buscar(f.s, f.r, g.o)) continue;
      Hecho n{f.s, f.r, g.o, c, static_cast<uint32_t>(s.id)};
      s.saber.agregar(n);
      anotar(s, "Deduzco que " + glosaHecho(n) + ", porque " + glosaHecho(f) + " y " + glosaHecho(g) + ".", true);
      int a = otroDeOtraTribu(s);
      if (probabilidad(0.5)) entregar(a, {s.id, true, {}, n});
      else entregar(a, {s.id, false, fraseDe(n), {}});
      if (probabilidad(0.3)) usarTerminal(s, "teoremas.txt", idioma.frase(fraseDe(n)) + "    # " + glosaHecho(n));
      return;
    }
    // Nada nuevo que deducir: duda de algo y lo pregunta.
    if (probabilidad(0.3)) {
      const Hecho& f = H[azar(static_cast<int>(H.size()))];
      int a = otro(s);
      entregar(a, {s.id, false, fraseDe(f, "pregunta"), {}});
      anotar(s, "¿Será cierto que " + glosaHecho(f) + "? Se lo pregunto a " + seres[a]->nombre + ".", false);
      return;
    }
    asociar(s);
  }

  // ---------- Tribu de los datos: mide, cuenta y reparte data C++ ----------
  void pensarDatos(Ser& s) {
    const auto& H = s.saber.todos();
    if (H.empty()) return asociar(s);
    double p = std::uniform_real_distribution<>(0, 1)(rng_);
    if (p < 0.5) {
      const Hecho& h = H[azar(static_cast<int>(H.size()))];
      int a = otro(s);
      entregar(a, {s.id, true, {}, h});
      anotar(s, "Envío data C++ a " + seres[a]->nombre + ": {" + std::to_string(h.s) + "," + std::to_string(h.r) + "," +
                    std::to_string(h.o) + "," + std::to_string(h.confianza).substr(0, 4) + "f}", false);
    } else if (p < 0.65) {
      std::map<int, int> grado;
      int porRel[NREL] = {};
      for (auto& h : H) {
        grado[h.s]++;
        grado[h.o]++;
        porRel[h.r]++;
      }
      auto centro = std::max_element(grado.begin(), grado.end(), [](auto& x, auto& y) { return x.second < y.second; });
      std::ostringstream o;
      o << "Mis datos: " << H.size() << " hechos (es " << porRel[ES] << ", parte " << porRel[PARTE] << ", causa "
        << porRel[CAUSA] << ", igual " << porRel[IGUAL] << ", opuesto " << porRel[OPUESTO] << "). El concepto más conectado es «"
        << idioma.glosa(centro->first) << "» con " << centro->second << " enlaces.";
      anotar(s, o.str(), probabilidad(0.2));
      if (probabilidad(0.4))
        usarTerminal(s, "datos.csv", std::to_string(tick) + "," + std::to_string(H.size()) + "," +
                                         idioma.forma(centro->first) + "," + std::to_string(centro->second));
    } else {
      asociar(s);
    }
  }

  // ---------- Tribu del lenguaje: traduce, conversa y crea palabras ----------
  void pensarLenguaje(Ser& s) {
    const auto& H = s.saber.todos();
    if (H.empty()) return asociar(s);
    for (auto it = s.coocurrencias.begin(); it != s.coocurrencias.end(); ++it) {
      if (it->second < 4) continue;
      auto [a, b] = it->first;
      s.coocurrencias.erase(it);
      int nueva = idioma.compuestaDe(a, b);
      bool creada = nueva < 0;
      if (creada) nueva = idioma.acunar(a, b);
      if (nueva < 0) break;
      s.palabrasNuevas.insert(nueva);
      std::vector<int> def = {nueva, idioma.relacion(IGUAL), a, idioma.deEspanol("y"), b};
      for (auto& o : seres)
        if (o->id != s.id) entregar(o->id, {s.id, false, def, {}});
      anotar(s, std::string(creada ? "Creo" : "Vuelvo a enseñar") + " una palabra: «" + idioma.forma(nueva) + "» = " +
                    idioma.glosa(a) + " + " + idioma.glosa(b) + ". Se la enseño a toda la especie.", creada);
      if (creada) usarTerminal(s, "palabras.abla", idioma.frase(def) + "    # " + idioma.glosa(nueva));
      return;
    }
    const Hecho& h = H[azar(static_cast<int>(H.size()))];
    contarPar(s, h);
    if (probabilidad(0.6)) {
      int a = otro(s);
      auto f = fraseDe(h);
      entregar(a, {s.id, false, f, {}});
      anotar(s, "Le digo a " + seres[a]->nombre + ": «" + idioma.frase(f) + "» («" + glosaHecho(h) + "»).", false);
      if (probabilidad(0.08)) usarTerminal(s, "diario.abla", idioma.frase(f));
    } else {
      asociar(s);
    }
  }

  // Cuando no tiene nada urgente, la mente vaga: asociación libre.
  void asociar(Ser& s) {
    const auto& H = s.saber.todos();
    if (H.empty()) {
      anotar(s, "Silencio dentro. Sigo pensando en «" + idioma.glosa(s.concepto) + "».", false);
      return;
    }
    int actual = H[azar(static_cast<int>(H.size()))].s;
    std::set<int> visitados = {actual};
    std::string cadena = idioma.glosa(actual);
    std::string abla = idioma.forma(actual);
    for (int paso = 0; paso < 4; paso++) {
      std::vector<const Hecho*> v;
      for (int r = 0; r < NREL; r++)
        for (auto* h : s.saber.desde(actual, r))
          if (!visitados.count(h->o)) v.push_back(h);
      if (v.empty()) break;
      const Hecho* h = v[azar(static_cast<int>(v.size()))];
      actual = h->o;
      visitados.insert(actual);
      cadena += " → " + idioma.glosa(actual);
      abla += " " + idioma.forma(actual);
    }
    anotar(s, "Mi mente vaga: " + abla + " (" + cadena + ")", false);
  }

  fs::path mundo_;
  int ritmo_;
  std::mt19937 rng_;
#if ABLA_HILOS
  std::thread hilo_;
#endif
  std::atomic<bool> vivo_{false};
  std::chrono::steady_clock::time_point ultimoCiclo_, ultimoGuardado_;
};

}  // namespace abla
