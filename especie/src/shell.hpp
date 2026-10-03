#pragma once
// Terminal estilo bash/Termux, encerrada en la carpeta del mundo.
// La usan tanto el humano como los 27 seres (cada uno con su $HOME).

#include <algorithm>
#include <filesystem>
#include <fstream>
#include <optional>
#include <sstream>
#include <stdexcept>
#include <string>
#include <vector>

namespace abla {

namespace fs = std::filesystem;

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
  Shell(const fs::path& raiz, const fs::path& home)
      : raiz_(fs::weakly_canonical(fs::absolute(raiz))), home_(fs::weakly_canonical(fs::absolute(home))), cwd_(home_) {
    fs::create_directories(home_);
  }

  const fs::path& cwd() const { return cwd_; }
  const fs::path& home() const { return home_; }

  std::string mostrar(const fs::path& p) const {
    if (p == home_ && home_ != raiz_) return "~";
    auto rel = p.lexically_relative(raiz_);
    std::string s = rel.generic_string();
    return (s == "." || s.empty()) ? "/" : "/" + s;
  }

  // Ruta del usuario → ruta real; falla si intenta salir del mundo.
  fs::path resolver(const std::string& arg) const {
    fs::path p;
    if (arg.empty() || arg == "~") p = home_;
    else if (arg.rfind("~/", 0) == 0) p = home_ / arg.substr(2);
    else if (arg[0] == '/') p = raiz_ / arg.substr(1);
    else p = cwd_ / arg;
    p = p.lexically_normal();
    if (!p.has_filename() && p != raiz_) p = p.parent_path();
    auto rel = p.lexically_relative(raiz_);
    if (rel.empty() || *rel.begin() == "..") throw std::runtime_error(arg + ": fuera del mundo");
    return p;
  }

  // nullopt = no es un comando de la terminal.
  std::optional<std::string> ejecutar(const std::vector<std::string>& a) {
    if (a.empty()) return std::string();
    try {
      const std::string& c = a[0];
      if (c == "pwd") return mostrar(cwd_) == "~" ? mostrar(home_) + "\n" : mostrar(cwd_) + "\n";
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
    } catch (const std::exception& e) {
      return a[0] + ": " + e.what() + "\n";
    }
  }

 private:
  std::string cd(const std::vector<std::string>& a) {
    fs::path p = resolver(a.size() > 1 ? a[1] : "~");
    if (!fs::is_directory(p)) return "cd: " + (a.size() > 1 ? a[1] : "~") + ": no es un directorio\n";
    cwd_ = p;
    return "";
  }

  std::string ls(const std::vector<std::string>& a) {
    bool largo = false;
    std::string ruta = ".";
    for (size_t i = 1; i < a.size(); i++) {
      if (a[i] == "-l" || a[i] == "-la" || a[i] == "-al") largo = true;
      else if (a[i][0] != '-') ruta = a[i];
    }
    fs::path p = resolver(ruta);
    if (!fs::exists(p)) return "ls: " + ruta + ": no existe\n";
    if (!fs::is_directory(p)) return p.filename().string() + "\n";
    std::vector<fs::directory_entry> e(fs::directory_iterator(p), {});
    std::sort(e.begin(), e.end(), [](auto& x, auto& y) { return x.path().filename() < y.path().filename(); });
    std::ostringstream out;
    for (auto& d : e) {
      std::string n = d.path().filename().string() + (d.is_directory() ? "/" : "");
      if (largo) {
        std::string t = d.is_directory() ? "-" : std::to_string(d.file_size());
        out << (d.is_directory() ? "d " : "- ") << std::string(t.size() < 10 ? 10 - t.size() : 0, ' ') << t << "  " << n << "\n";
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
      fs::path p = resolver(a[i]);
      if (!fs::is_regular_file(p)) { s += "cat: " + a[i] + ": no es un archivo\n"; continue; }
      std::ifstream in(p);
      std::stringstream b;
      b << in.rdbuf();
      s += b.str();
    }
    return s;
  }

  std::string cabeza(const std::vector<std::string>& a, bool cola) {
    size_t n = 10;
    std::string ruta;
    for (size_t i = 1; i < a.size(); i++) {
      if (a[i] == "-n" && i + 1 < a.size()) n = std::stoul(a[++i]);
      else ruta = a[i];
    }
    fs::path p = resolver(ruta);
    if (!fs::is_regular_file(p)) return a[0] + ": " + ruta + ": no es un archivo\n";
    std::ifstream in(p);
    std::vector<std::string> lineas;
    std::string l;
    while (std::getline(in, l)) {
      lineas.push_back(l);
      if (!cola && lineas.size() >= n) break;
      if (cola && lineas.size() > n) lineas.erase(lineas.begin());
    }
    std::string s;
    for (auto& x : lineas) s += x + "\n";
    return s;
  }

  std::string mkdir(const std::vector<std::string>& a) {
    for (size_t i = 1; i < a.size(); i++)
      if (a[i] != "-p") fs::create_directories(resolver(a[i]));
    return "";
  }

  std::string touch(const std::vector<std::string>& a) {
    for (size_t i = 1; i < a.size(); i++) std::ofstream(resolver(a[i]), std::ios::app);
    return "";
  }

  std::string rm(const std::vector<std::string>& a) {
    bool r = false;
    std::string s;
    for (size_t i = 1; i < a.size(); i++) {
      if (a[i] == "-r" || a[i] == "-rf" || a[i] == "-f") { r = true; continue; }
      fs::path p = resolver(a[i]);
      if (p == raiz_ || p == home_) { s += "rm: no se puede borrar " + a[i] + "\n"; continue; }
      if (!fs::exists(p)) { s += "rm: " + a[i] + ": no existe\n"; continue; }
      if (fs::is_directory(p) && !r) { s += "rm: " + a[i] + ": es un directorio (usa -r)\n"; continue; }
      fs::remove_all(p);
    }
    return s;
  }

  std::string mover(const std::vector<std::string>& a, bool copiar) {
    if (a.size() != 3) return a[0] + ": uso: " + a[0] + " origen destino\n";
    fs::path o = resolver(a[1]), d = resolver(a[2]);
    if (fs::is_directory(d)) d /= o.filename();
    if (copiar) fs::copy(o, d, fs::copy_options::recursive | fs::copy_options::overwrite_existing);
    else fs::rename(o, d);
    return "";
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
    std::ofstream(resolver(destino), anexar ? std::ios::app : std::ios::trunc) << texto << "\n";
    return "";
  }

  std::string tree(const std::vector<std::string>& a) {
    fs::path p = resolver(a.size() > 1 ? a[1] : ".");
    std::string s = mostrar(p) + "\n";
    int cuenta = 0;
    arbol(p, "", 0, s, cuenta);
    return s;
  }
  void arbol(const fs::path& p, const std::string& pre, int nivel, std::string& s, int& cuenta) {
    if (nivel > 3) return;
    std::vector<fs::directory_entry> e(fs::directory_iterator(p), {});
    std::sort(e.begin(), e.end(), [](auto& x, auto& y) { return x.path().filename() < y.path().filename(); });
    for (size_t i = 0; i < e.size(); i++) {
      if (++cuenta > 400) { s += pre + "…\n"; return; }
      bool ultimo = i + 1 == e.size();
      s += pre + (ultimo ? "└── " : "├── ") + e[i].path().filename().string() + (e[i].is_directory() ? "/" : "") + "\n";
      if (e[i].is_directory()) arbol(e[i].path(), pre + (ultimo ? "    " : "│   "), nivel + 1, s, cuenta);
    }
  }

  std::string wc(const std::vector<std::string>& a) {
    std::string s;
    for (size_t i = 1; i < a.size(); i++) {
      std::ifstream in(resolver(a[i]));
      size_t l = 0, w = 0, b = 0;
      std::string x;
      while (std::getline(in, x)) {
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
      std::ifstream in(resolver(a[i]));
      std::string x;
      while (std::getline(in, x))
        if (x.find(a[1]) != std::string::npos) s += (a.size() > 3 ? a[i] + ":" : "") + x + "\n";
    }
    return s;
  }

  fs::path raiz_, home_, cwd_;
};

}  // namespace abla
