// Especie Abla — 27 mentes que no dejan de pensar.
//
// Compilar:  g++ -std=c++17 -O2 -pthread src/main.cpp -o abla
// Sin hilos (Code App en iPad): se detecta solo, o con -DABLA_SIN_HILOS
// Usar:      ./abla [--mundo DIR] [--contexto TOKENS] [--ritmo MS]

#if __cplusplus < 201703L
#error "Abla necesita C++17. En Code App no uses el boton de play: en la TERMINAL escribe  cd especie  y luego  clang++ -std=c++17 -O2 src/main.cpp -o abla  y despues  wasm abla"
#endif

#include <cstdlib>
#include <iostream>

#include "especie.hpp"

using namespace abla;

namespace {

bool color = std::getenv("NO_COLOR") == nullptr;
std::string c(const char* codigo, const std::string& s) { return color ? "\033[" + std::string(codigo) + "m" + s + "\033[0m" : s; }

const char* AYUDA = R"(
Terminal (estilo bash, dentro de mundo/):
  ls [-l] [ruta]   cd [ruta]   pwd   cat f   head/tail [-n N] f   wc f   grep patrón f
  mkdir d   touch f   rm [-r] f   mv a b   cp a b   tree [ruta]   echo texto [> | >> f]
  clear   whoami   history   exit

La especie (ser = número 1-27 o nombre):
  seres                         los 27 y qué están pensando
  ser <ser>                     estado: memoria, contexto, saber, mensajes
  mente <ser> [n]               sus últimos n pensamientos (contexto)
  recordar <ser> <texto>        busca en toda su memoria, también la de largo plazo
  escuchar [segundos] [ser]     escucha a la especie pensar en vivo
  terminal <ser>                lo que el ser ha hecho en su propia terminal
  decir <ser|todos> <texto>     háblale en Abla o en español (raíces del diccionario)
                                p. ej.  decir 3 sol causa luz pregunta
  enseñar <ser|todos> <a> <rel> <b>   manda data C++ exacta; rel = es|parte|causa|igual|opuesto
  alimentar <ser|todos> <archivo>     manda un archivo de líneas «a rel b» como data C++
  dic <palabra>                 traduce entre Abla y español
  dic buscar <texto>            busca en el diccionario
  traducir <frase>              traduce una frase de Abla
  idioma                        estado del idioma (4000 innatas + creadas)
  red                           mensajes entre los seres
  guardar                       guarda ahora (también se guarda solo cada 30 s)
)";

std::string unir(const std::vector<std::string>& a, size_t desde) {
  std::string s;
  for (size_t i = desde; i < a.size(); i++) s += (s.empty() ? "" : " ") + a[i];
  return s;
}

std::vector<Ser*> destinatarios(Especie& e, const std::string& q) {
  std::vector<Ser*> v;
  if (q == "todos")
    for (auto& s : e.seres) v.push_back(s.get());
  else if (Ser* s = e.buscarSer(q))
    v.push_back(s);
  return v;
}

std::string cortar(const std::string& s, size_t n) {
  // corta sin partir caracteres UTF-8
  size_t cuenta = 0, i = 0;
  for (; i < s.size() && cuenta < n; i++)
    if ((static_cast<unsigned char>(s[i]) & 0xC0) != 0x80) cuenta++;
  while (i < s.size() && (static_cast<unsigned char>(s[i]) & 0xC0) == 0x80) i++;
  return i < s.size() ? s.substr(0, i) + "…" : s;
}

void cmdSeres(Especie& e) {
  for (auto& s : e.seres) {
    std::cout << c("1", (s->id < 9 ? " " : "") + std::to_string(s->id + 1)) << " "
              << c(s->tribu == Tribu::LENGUAJE ? "35" : s->tribu == Tribu::DATOS ? "36" : "32", s->nombre) << " "
              << std::string(9 - std::min<size_t>(9, s->nombre.size()), ' ') << nombreTribu(s->tribu) << "\t"
              << s->saber.size() << " hechos\t" << cortar(s->ultimo, 70) << "\n";
  }
}

void cmdSer(Especie& e, Ser& s) {
  auto& m = s.memoria;
  std::cout << c("1", s.nombre) << " — «quien " << e.idioma.glosa(s.concepto) << "», tribu " << nombreTribu(s.tribu) << "\n"
            << "  pensamientos:     " << s.pensamientos << " (no se detiene)\n"
            << "  contexto:         " << m.usados() << " / " << m.capacidad() << " tokens (" << m.contexto().size()
            << " recuerdos presentes)\n"
            << "  largo plazo:      " << m.largoPlazo() << " recuerdos, " << m.bytesEnDisco() / 1024 << " KB en "
            << fs::relative(m.archivo(), e.mundo()).generic_string() << "\n"
            << "  conocimiento:     " << s.saber.size() << " hechos en "
            << fs::relative(e.archivoCpp(s), e.mundo()).generic_string() << "\n"
            << "  idioma:           " << Idioma::BASE << " innatas + " << s.palabrasNuevas.size() << " nuevas\n"
            << "  mensajes:         " << s.enviados << " enviados, " << s.recibidos << " recibidos, " << s.buzon.size()
            << " en el buzón\n"
            << "  terminal:         /seres/" << s.nombre << "\n"
            << "  ahora piensa:     " << s.ultimo << "\n";
}

void cmdEscuchar(Especie& e, int segundos, const std::string& filtro) {
  uint64_t visto;
  {
    Guardia l(e.mtx);
    visto = e.nLinea > 10 ? e.nLinea - 10 : 0;
  }
  std::cout << c("2", "(escuchando " + std::to_string(segundos) + " s…)") << "\n";
  if (!Especie::conHilos) {  // sin hilos: piensa ahora esos segundos de golpe
    Guardia l(e.mtx);
    for (int i = 0, n = std::min(400, segundos * 1000 / e.ritmo()); i < n; i++) e.ciclo();
    for (auto& ln : e.corriente)
      if (ln.n > visto && (filtro.empty() || ln.texto.find(filtro) != std::string::npos)) std::cout << ln.texto << "\n";
    return;
  }
#if ABLA_HILOS
  auto fin = std::chrono::steady_clock::now() + std::chrono::seconds(segundos);
  while (std::chrono::steady_clock::now() < fin) {
    {
      Guardia l(e.mtx);
      for (auto& ln : e.corriente)
        if (ln.n > visto && (filtro.empty() || ln.texto.find(filtro) != std::string::npos)) std::cout << ln.texto << "\n";
      visto = e.nLinea;
    }
    std::cout.flush();
    std::this_thread::sleep_for(std::chrono::milliseconds(250));
  }
#endif
}

}  // namespace

int main(int argc, char** argv) {
  std::string mundo = "mundo";
  uint64_t contexto = 1'000'000;
  int ritmo = 800;
  for (int i = 1; i < argc; i++) {
    std::string a = argv[i];
    if (a == "--mundo" && i + 1 < argc) mundo = argv[++i];
    else if (a == "--contexto" && i + 1 < argc) contexto = std::stoull(argv[++i]);
    else if (a == "--ritmo" && i + 1 < argc) ritmo = std::max(50, std::atoi(argv[++i]));
    else {
      std::cout << "uso: " << argv[0] << " [--mundo DIR] [--contexto TOKENS] [--ritmo MS]\n";
      return a == "-h" || a == "--help" ? 0 : 1;
    }
  }

  std::cout << c("1;32", "Despertando a la especie…") << "\n";
  Especie e(mundo, contexto, ritmo);
  e.despertar();
  Shell yo(e.mundo(), e.mundo());

  {
    Guardia l(e.mtx);
    int hola = e.idioma.deEspanol("despertar"), noso = e.idioma.deEspanol("nosotros"), pens = e.idioma.deEspanol("pensamiento");
    std::cout << "\n  " << c("1", "Especie Abla") << " · 27 seres · " << e.idioma.tamano() << " palabras · contexto "
              << contexto << " tokens por ser\n"
              << "  " << c("35", e.idioma.frase({noso, hola, pens})) << "  («" << e.idioma.glosaFrase({noso, hola, pens})
              << "»)\n\n  Escribe " << c("1", "ayuda") << " para ver los comandos. Ellos ya están pensando.\n\n";
  }

  std::vector<std::string> historial;
  std::string linea;
  while (true) {
    std::cout << c("1;32", "humano@abla") << ":" << c("1;34", yo.mostrar(yo.cwd())) << "$ " << std::flush;
    if (!std::getline(std::cin, linea)) break;
    auto a = trocear(linea);
    if (a.empty()) continue;
    historial.push_back(linea);
    const std::string& cmd = a[0];

    if (cmd == "exit" || cmd == "salir") break;
    if (cmd == "ayuda" || cmd == "help") { std::cout << AYUDA; continue; }
    if (cmd == "clear") { std::cout << "\033[2J\033[H"; continue; }
    if (cmd == "whoami") { std::cout << "humano\n"; continue; }
    if (cmd == "history") {
      for (size_t i = 0; i < historial.size(); i++) std::cout << "  " << i + 1 << "  " << historial[i] << "\n";
      continue;
    }
    if (cmd == "escuchar") {
      int seg = a.size() > 1 && std::isdigit(static_cast<unsigned char>(a[1][0])) ? std::atoi(a[1].c_str()) : 10;
      std::string filtro;
      if (a.size() > 1 && !std::isdigit(static_cast<unsigned char>(a[1][0]))) filtro = a[1];
      if (a.size() > 2) filtro = a[2];
      if (!filtro.empty()) {
        Guardia l(e.mtx);
        if (Ser* s = e.buscarSer(filtro)) filtro = s->nombre;
      }
      cmdEscuchar(e, std::max(1, seg), filtro);
      continue;
    }

    Guardia l(e.mtx);
    e.ponerseAlDia();
    if (cmd == "seres") { cmdSeres(e); continue; }
    if (cmd == "ser" || cmd == "mente" || cmd == "terminal" || cmd == "recordar") {
      Ser* s = a.size() > 1 ? e.buscarSer(a[1]) : nullptr;
      if (!s) { std::cout << cmd << ": ¿qué ser? (1-27 o nombre)\n"; continue; }
      if (cmd == "ser") cmdSer(e, *s);
      else if (cmd == "mente") {
        size_t n = a.size() > 2 ? std::stoul(a[2]) : 15;
        auto& ctx = s->memoria.contexto();
        for (size_t i = ctx.size() > n ? ctx.size() - n : 0; i < ctx.size(); i++)
          std::cout << c("2", "[t=" + std::to_string(ctx[i].tick) + "] ") << ctx[i].texto << "\n";
      } else if (cmd == "terminal") {
        if (s->historial.empty()) std::cout << "(todavía no ha usado su terminal)\n";
        for (auto& h : s->historial) std::cout << h << "\n";
      } else {
        auto r = s->memoria.buscar(unir(a, 2), 20);
        if (r.empty()) std::cout << "(no lo recuerda)\n";
        for (auto& x : r) std::cout << x << "\n";
      }
      continue;
    }
    if (cmd == "decir" || cmd == "enseñar" || cmd == "ensenar" || cmd == "alimentar") {
      auto para = a.size() > 1 ? destinatarios(e, a[1]) : std::vector<Ser*>{};
      if (para.empty()) { std::cout << cmd << ": ¿a quién? (1-27, nombre o todos)\n"; continue; }
      std::vector<Hecho> datos;
      if (cmd == "decir") {
        std::vector<std::string> raras;
        auto f = e.idioma.leer(unir(a, 2), &raras);
        if (!raras.empty()) std::cout << c("2", "(no son palabras de Abla ni raíces: " + unir(raras, 0) + ")") << "\n";
        if (f.empty()) continue;
        std::cout << "En Abla: " << c("35", e.idioma.frase(f)) << "  («" << e.idioma.glosaFrase(f) << "»)\n";
        for (Ser* s : para) e.entregar(s->id, {DE_HUMANO, false, f, {}});
      } else {
        std::vector<std::vector<std::string>> lineas;
        if (cmd == "alimentar") {
          try {
            std::ifstream in(yo.resolver(a.size() > 2 ? a[2] : ""));
            if (!in) throw std::runtime_error("no se puede leer el archivo");
            std::string x;
            while (std::getline(in, x)) lineas.push_back(trocear(x));
          } catch (const std::exception& ex) { std::cout << "alimentar: " << ex.what() << "\n"; continue; }
        } else {
          lineas.push_back(std::vector<std::string>(a.begin() + 2, a.end()));
        }
        int malas = 0;
        for (auto& x : lineas) {
          if (x.empty() || x[0][0] == '#') continue;
          int s = x.size() == 3 ? e.idioma.deForma(x[0]) : -1, o = x.size() == 3 ? e.idioma.deForma(x[2]) : -1;
          if (x.size() == 3 && s < 0) s = e.idioma.deEspanol(x[0]);
          if (x.size() == 3 && o < 0) o = e.idioma.deEspanol(x[2]);
          int r = x.size() == 3 ? relDeNombre(x[1]) : -1;
          if (s < 0 || o < 0 || r < 0) { malas++; continue; }
          datos.push_back({static_cast<uint16_t>(s), static_cast<uint8_t>(r), static_cast<uint16_t>(o), 1.0f, FUENTE_HUMANO});
        }
        for (auto& h : datos)
          for (Ser* s : para) e.entregar(s->id, {DE_HUMANO, true, {}, h});
        std::cout << "Enviada data C++: " << datos.size() << " hechos a " << para.size() << " ser(es)";
        if (malas) std::cout << " (" << malas << " líneas no entendidas; formato: a es|parte|causa|igual|opuesto b)";
        std::cout << "\n";
        if (datos.empty()) continue;
      }
      if (para.size() == 1) {  // respuesta inmediata
        Ser& s = *para[0];
        s.respuesta.clear();
        // te atiende primero: tu mensaje pasa al frente de su buzón
        auto tuyo = std::find_if(s.buzon.rbegin(), s.buzon.rend(), [](const Mensaje& m) { return m.de == DE_HUMANO; });
        if (tuyo != s.buzon.rend()) {
          Mensaje m = std::move(*tuyo);
          s.buzon.erase(std::next(tuyo).base());
          s.buzon.push_front(std::move(m));
        }
        e.pensar(s);
        std::cout << c("1", s.nombre) << ": " << (s.respuesta.empty() ? s.ultimo : s.respuesta) << "\n";
      }
      continue;
    }
    if (cmd == "dic") {
      if (a.size() > 2 && a[1] == "buscar") {
        std::string q = minusculas(unir(a, 2));
        int n = 0;
        for (int i = 0; i < e.idioma.tamano() && n < 40; i++)
          if (e.idioma.glosa(i).find(q) != std::string::npos || e.idioma.forma(i).find(q) != std::string::npos) {
            std::cout << "  " << e.idioma.forma(i) << "\t" << e.idioma.glosa(i) << "\n";
            n++;
          }
        if (!n) std::cout << "(nada)\n";
      } else if (a.size() > 1) {
        int i = e.idioma.deForma(a[1]);
        if (i >= 0) std::cout << a[1] << " = " << e.idioma.glosa(i) << "\n";
        else if ((i = e.idioma.deEspanol(a[1])) >= 0) {
          std::cout << a[1] << " = " << c("35", e.idioma.forma(i)) << "   (aspectos: ";
          for (int k = 1; k < Idioma::ASPECTOS; k++)
            std::cout << e.idioma.forma(i + k) << "=" << aspectos()[k] << (k + 1 < Idioma::ASPECTOS ? ", " : ")\n");
        } else std::cout << "dic: «" << a[1] << "» no está en Abla (prueba: dic buscar " << a[1] << ")\n";
      } else std::cout << "uso: dic <palabra> | dic buscar <texto>\n";
      continue;
    }
    if (cmd == "traducir") {
      auto f = e.idioma.leer(unir(a, 1));
      std::cout << e.idioma.glosaFrase(f) << "\n";
      continue;
    }
    if (cmd == "idioma") {
      std::cout << "Abla: " << e.idioma.tamano() << " palabras = " << Idioma::BASE << " innatas (" << Idioma::RAICES
                << " raíces × " << Idioma::ASPECTOS << " aspectos) + " << e.idioma.compuestas() << " creadas por la especie\n"
                << "Diccionario completo: /abla/diccionario.txt\n";
      for (int i = std::max(Idioma::BASE, e.idioma.tamano() - 8); i < e.idioma.tamano(); i++)
        std::cout << "  nueva: " << c("35", e.idioma.forma(i)) << " = " << e.idioma.glosa(i) << "\n";
      continue;
    }
    if (cmd == "red") {
      std::cout << "Ciclo " << e.tick << " · mensajes en Abla: " << e.mensajesAbla << " · data C++: " << e.mensajesCpp << "\n";
      continue;
    }
    if (cmd == "guardar") {
      e.guardar();
      std::cout << "Guardado: conocimiento en /conocimiento/*.cpp, memoria en /memoria/\n";
      continue;
    }
    auto out = yo.ejecutar(a);
    if (out) std::cout << *out;
    else std::cout << cmd << ": comando no encontrado (escribe ayuda)\n";
  }
  std::cout << "\nGuardando… La especie dormirá hasta que vuelvas a abrirla, y recordará todo.\n";
  return 0;
}
