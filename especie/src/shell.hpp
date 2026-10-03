#pragma once
// Terminal estilo bash/Termux, encerrada en la carpeta del mundo.
// La usan tanto el humano como los 27 seres (cada uno con su $HOME).
//
// Trabaja con rutas "virtuales" que empiezan en "/" (la raíz del mundo);
// nunca se puede salir de ahí. Sin <filesystem> ni excepciones.

#include <cstdlib>
#include <optional>
#include <sstream>
#include <string>
#include <vector>

#include "archivos.hpp"

namespace abla {

// Divide una línea respetando "comillas" y 'comillas'.
inline std::vector<std::string> trocear(const std::string& linea) {
  std::vector<std::string> out;
  std::string actual;
  char comilla = 0;
  bool hay = false;
  for (char c : linea) {
    if (comilla) {
      if (c == comilla) comilla = 0;
      else actual += c;
    } else if (c == '"' || c == '\'') {
      comilla = c;
      hay = true;
    } else if (c == ' ' || c == '\t') {
      if (hay || !actual.empty()) out.push_back(actual);
      actual.clear();
      hay = false;
    } else {
      actual += c;
    }
  }
  if (hay || !actual.empty()) out.push_back(actual);
  return out;
}

class Shell {
 public:
  // raiz: carpeta real del mundo (p. ej. "mundo"); home: ruta virtual (p. ej. "/seres/Tuteje").
  Shell(const std::string& raiz, const std::string& home) : raiz_(raiz), home_(home), cwd_(home) {
    archivos::crearDirectorios(real(home_));
  }

  const std::string& cwd() const { return cwd_; }
  const std::string& home() const { return home_; }
  std::string real(const std::string& virtual_) const { return virtual_ == "/" ? raiz_ : archivos::unir(raiz_, virtual_); }
  std::string mostrar(const std::string& v) const { return v == home_ && home_ != "/" ? "~" : v; }

  // Ruta escrita por el usuario → ruta virtual normalizada. false si intenta salir del mundo.
  bool resolver(const std::string& arg, std::string& out) const {
    std::string base, resto = arg;
    if (arg.empty() || arg == "~") resto = home_;
    else if (arg.rfind("~/", 0) == 0) resto = home_ + arg.substr(1);
    else if (arg[0] != '/') base = cwd_;
    std::vector<std::string> partes;
    for (const std::string* s : {&base, &resto}) {
      std::stringstream ss(*s);
      std::string p;
      while (std::getline(ss, p, '/')) {
        if (p.empty() || p == ".") continue;
        if (p == "..") {
          if (partes.empty()) return false;
          partes.pop_back();
        } else {
          partes.push_back(p);
        }
      }
    }
    out.clear();
    for (auto& p : partes) out += "/" + p;
    if (out.empty()) out = "/";
    return true;
  }

  // nullopt = no es un comando de la terminal.
  std::optional<std::string> ejecutar(const std::vector<std::string>& a) {
    if (a.empty()) return std::string();
    const std::string& c = a[0];
    if (c == "pwd") return cwd_ + "\n";
    if (c == "cd") return cd(a);
    if (c == "ls") return ls(a);
    if (c == "cat") return cat(a);
    if (c == "head" || c == "tail") return cabeza(a, c == "tail");
    if (c == "mkdir") return mkdir(a);
    if (c == "touch") return touch(a);
    if (c == "rm") return rm(a);
    if (c == "mv" || c == "cp") return mover(a, c == "cp");
    if (c == "echo") return echo(a);
    if (c == "tree") return tree(a);
    if (c == "wc") return wc(a);
    if (c == "grep") return grep(a);
    return std::nullopt;
  }

 private:
  // Resuelve y deja la ruta real en `r`; si falla, deja el error en `err`.
  bool ruta(const std::string& cmd, const std::string& arg, std::string& r, std::string& err, std::string* v = nullptr) const {
    std::string virt;
    if (!resolver(arg, virt)) {
      err += cmd + ": " + arg + ": fuera del mundo\n";
      return false;
    }
    if (v) *v = virt;
    r = real(virt);
    return true;
  }

  std::string cd(const std::vector<std::string>& a) {
    std::string arg = a.size() > 1 ? a[1] : "~", r, err, v;
    if (!ruta("cd", arg, r, err, &v)) return err;
    if (!archivos::esDirectorio(r)) return "cd: " + arg + ": no es un directorio\n";
    cwd_ = v;
    return "";
  }

  std::string ls(const std::vector<std::string>& a) {
    bool largo = false;
    std::string arg = ".", r, err;
    for (size_t i = 1; i < a.size(); i++) {
      if (a[i] == "-l" || a[i] == "-la" || a[i] == "-al") largo = true;
      else if (a[i][0] != '-') arg = a[i];
    }
    if (!ruta("ls", arg, r, err)) return err;
    if (!archivos::existe(r)) return "ls: " + arg + ": no existe\n";
    if (!archivos::esDirectorio(r)) return archivos::nombre(r) + "\n";
    std::ostringstream out;
    for (auto& e : archivos::listar(r)) {
      std::string n = e.nombre + (e.directorio ? "/" : "");
      if (largo) {
        std::string t = e.directorio ? "-" : std::to_string(e.tamano);
        out << (e.directorio ? "d " : "- ") << std::string(t.size() < 10 ? 10 - t.size() : 0, ' ') << t << "  " << n << "\n";
      } else {
        out << n << "  ";
      }
    }
    std::string s = out.str();
    if (!largo && !s.empty()) s += "\n";
    return s;
  }

  std::string cat(const std::vector<std::string>& a) {
    std::string s;
    for (size_t i = 1; i < a.size(); i++) {
      std::string r;
      if (!ruta("cat", a[i], r, s)) continue;
      if (!archivos::esArchivo(r)) { s += "cat: " + a[i] + ": no es un archivo\n"; continue; }
      std::string b;
      archivos::leer(r, b);
      s += b;
    }
    return s;
  }

  std::string cabeza(const std::vector<std::string>& a, bool cola) {
    size_t n = 10;
    std::string arg, r, err;
    for (size_t i = 1; i < a.size(); i++) {
      if (a[i] == "-n" && i + 1 < a.size()) n = std::strtoul(a[++i].c_str(), nullptr, 10);
      else arg = a[i];
    }
    if (!ruta(a[0], arg, r, err)) return err;
    if (!archivos::esArchivo(r)) return a[0] + ": " + arg + ": no es un archivo\n";
    archivos::Lector in(r);
    std::vector<std::string> lineas;
    std::string l;
    while (in.linea(l)) {
      lineas.push_back(l);
      if (!cola && lineas.size() >= n) break;
      if (cola && lineas.size() > n) lineas.erase(lineas.begin());
    }
    std::string s;
    for (auto& x : lineas) s += x + "\n";
    return s;
  }

  std::string mkdir(const std::vector<std::string>& a) {
    std::string err;
    for (size_t i = 1; i < a.size(); i++) {
      std::string r;
      if (a[i] != "-p" && ruta("mkdir", a[i], r, err) && !archivos::crearDirectorios(r))
        err += "mkdir: " + a[i] + ": no se pudo crear\n";
    }
    return err;
  }

  std::string touch(const std::vector<std::string>& a) {
    std::string err;
    for (size_t i = 1; i < a.size(); i++) {
      std::string r;
      if (ruta("touch", a[i], r, err)) archivos::escribir(r, "", true);
    }
    return err;
  }

  std::string rm(const std::vector<std::string>& a) {
    bool recursivo = false;
    std::string s;
    for (size_t i = 1; i < a.size(); i++) {
      if (a[i] == "-r" || a[i] == "-rf" || a[i] == "-f") { recursivo = true; continue; }
      std::string r, v;
      if (!ruta("rm", a[i], r, s, &v)) continue;
      if (v == "/" || v == home_) { s += "rm: no se puede borrar " + a[i] + "\n"; continue; }
      if (!archivos::existe(r)) { s += "rm: " + a[i] + ": no existe\n"; continue; }
      if (archivos::esDirectorio(r) && !recursivo) { s += "rm: " + a[i] + ": es un directorio (usa -r)\n"; continue; }
      archivos::borrarTodo(r);
    }
    return s;
  }

  std::string mover(const std::vector<std::string>& a, bool copiar) {
    if (a.size() != 3) return a[0] + ": uso: " + a[0] + " origen destino\n";
    std::string o, d, err;
    if (!ruta(a[0], a[1], o, err) || !ruta(a[0], a[2], d, err)) return err;
    if (!archivos::existe(o)) return a[0] + ": " + a[1] + ": no existe\n";
    if (archivos::esDirectorio(d)) d = archivos::unir(d, archivos::nombre(o));
    bool ok = copiar ? archivos::copiar(o, d) : archivos::renombrar(o, d);
    return ok ? "" : a[0] + ": no se pudo\n";
  }

  std::string echo(const std::vector<std::string>& a) {
    std::string texto, destino;
    bool anexar = false;
    for (size_t i = 1; i < a.size(); i++) {
      if ((a[i] == ">" || a[i] == ">>") && i + 1 < a.size()) {
        anexar = a[i] == ">>";
        destino = a[++i];
        continue;
      }
      texto += (texto.empty() ? "" : " ") + a[i];
    }
    if (destino.empty()) return texto + "\n";
    std::string r, err;
    if (!ruta("echo", destino, r, err)) return err;
    archivos::escribir(r, texto + "\n", anexar);
    return "";
  }

  std::string tree(const std::vector<std::string>& a) {
    std::string r, err, v;
    if (!ruta("tree", a.size() > 1 ? a[1] : ".", r, err, &v)) return err;
    std::string s = v + "\n";
    int cuenta = 0;
    arbol(r, "", 0, s, cuenta);
    return s;
  }
  void arbol(const std::string& p, const std::string& pre, int nivel, std::string& s, int& cuenta) {
    if (nivel > 3) return;
    auto e = archivos::listar(p);
    for (size_t i = 0; i < e.size(); i++) {
      if (++cuenta > 400) { s += pre + "…\n"; return; }
      bool ultimo = i + 1 == e.size();
      s += pre + (ultimo ? "└── " : "├── ") + e[i].nombre + (e[i].directorio ? "/" : "") + "\n";
      if (e[i].directorio) arbol(archivos::unir(p, e[i].nombre), pre + (ultimo ? "    " : "│   "), nivel + 1, s, cuenta);
    }
  }

  std::string wc(const std::vector<std::string>& a) {
    std::string s;
    for (size_t i = 1; i < a.size(); i++) {
      std::string r;
      if (!ruta("wc", a[i], r, s)) continue;
      archivos::Lector in(r);
      size_t l = 0, w = 0, b = 0;
      std::string x;
      while (in.linea(x)) {
        l++;
        b += x.size() + 1;
        std::istringstream ws(x);
        std::string y;
        while (ws >> y) w++;
      }
      s += std::to_string(l) + " " + std::to_string(w) + " " + std::to_string(b) + " " + a[i] + "\n";
    }
    return s;
  }

  std::string grep(const std::vector<std::string>& a) {
    if (a.size() < 3) return "grep: uso: grep patrón archivo...\n";
    std::string s;
    for (size_t i = 2; i < a.size(); i++) {
      std::string r;
      if (!ruta("grep", a[i], r, s)) continue;
      archivos::Lector in(r);
      std::string x;
      while (in.linea(x))
        if (x.find(a[1]) != std::string::npos) s += (a.size() > 3 ? a[i] + ":" : "") + x + "\n";
    }
    return s;
  }

  std::string raiz_, home_, cwd_;
};

}  // namespace abla
