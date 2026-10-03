#pragma once
// Archivos y carpetas sin <filesystem>, <fstream> ni excepciones: solo POSIX
// (stat, mkdir, opendir…) y stdio de C. Sin <sys/types.h> ni <unistd.h>: en Code App
// sys/types.h pide endian.h, que no existe. Así compila también en Code App
// (WebAssembly), cuya libc++ no trae <filesystem> ni <fstream> ni permite `throw`.
//
// Las rutas son std::string con '/' y pueden ser relativas (p. ej. "mundo").

#include <dirent.h>
#include <sys/stat.h>

#include <algorithm>
#include <cstdio>
#include <string>
#include <vector>

namespace abla {
namespace archivos {

// ---------- Leer y escribir con stdio ----------

inline bool leer(const std::string& p, std::string& out) {
  out.clear();
  std::FILE* f = std::fopen(p.c_str(), "rb");
  if (!f) return false;
  char buf[8192];
  size_t n;
  while ((n = std::fread(buf, 1, sizeof buf, f)) > 0) out.append(buf, n);
  std::fclose(f);
  return true;
}

inline bool escribir(const std::string& p, const std::string& datos, bool anexar = false) {
  std::FILE* f = std::fopen(p.c_str(), anexar ? "ab" : "wb");
  if (!f) return false;
  bool ok = std::fwrite(datos.data(), 1, datos.size(), f) == datos.size();
  return std::fclose(f) == 0 && ok;
}

// Lee un archivo línea a línea:  Lector in(ruta); while (in.linea(s)) ...
class Lector {
 public:
  explicit Lector(const std::string& p) : f_(std::fopen(p.c_str(), "rb")) {}
  ~Lector() {
    if (f_) std::fclose(f_);
  }
  Lector(const Lector&) = delete;
  Lector& operator=(const Lector&) = delete;
  explicit operator bool() const { return f_ != nullptr; }
  bool linea(std::string& s) {
    s.clear();
    if (!f_) return false;
    int c;
    bool algo = false;
    while ((c = std::fgetc(f_)) != EOF) {
      algo = true;
      if (c == '\n') return true;
      s += static_cast<char>(c);
    }
    return algo;
  }

 private:
  std::FILE* f_;
};

// Archivo abierto para ir añadiendo al final (la memoria de largo plazo).
class Anexador {
 public:
  Anexador() = default;
  ~Anexador() { cerrar(); }
  Anexador(const Anexador&) = delete;
  Anexador& operator=(const Anexador&) = delete;
  void abrir(const std::string& p) {
    cerrar();
    f_ = std::fopen(p.c_str(), "ab");
  }
  void escribir(const std::string& s) {
    if (f_) std::fwrite(s.data(), 1, s.size(), f_);
  }
  void vaciar() {
    if (f_) std::fflush(f_);
  }
  void cerrar() {
    if (f_) std::fclose(f_);
    f_ = nullptr;
  }

 private:
  std::FILE* f_ = nullptr;
};

// ---------- Rutas ----------

inline std::string unir(const std::string& a, const std::string& b) {
  if (a.empty() || b.empty()) return a + b;
  if (a.back() == '/') return b.front() == '/' ? a + b.substr(1) : a + b;
  return b.front() == '/' ? a + b : a + "/" + b;
}

inline std::string nombre(const std::string& p) {
  auto i = p.find_last_of('/');
  return i == std::string::npos ? p : p.substr(i + 1);
}

inline bool estado(const std::string& p, struct stat& st) { return ::stat(p.c_str(), &st) == 0; }
inline bool existe(const std::string& p) {
  struct stat st;
  return estado(p, st);
}
inline bool esDirectorio(const std::string& p) {
  struct stat st;
  return estado(p, st) && S_ISDIR(st.st_mode);
}
inline bool esArchivo(const std::string& p) {
  struct stat st;
  return estado(p, st) && S_ISREG(st.st_mode);
}
inline long long tamano(const std::string& p) {
  struct stat st;
  return estado(p, st) ? static_cast<long long>(st.st_size) : -1;
}

// mkdir -p
inline bool crearDirectorios(const std::string& p) {
  if (p.empty() || esDirectorio(p)) return true;
  for (size_t i = 1; i <= p.size(); i++) {
    if (i == p.size() || p[i] == '/') {
      std::string parcial = p.substr(0, i);
      if (!esDirectorio(parcial) && ::mkdir(parcial.c_str(), 0755) != 0 && !esDirectorio(parcial)) return false;
    }
  }
  return true;
}

struct Entrada {
  std::string nombre;
  bool directorio;
  long long tamano;
};

inline std::vector<Entrada> listar(const std::string& p) {
  std::vector<Entrada> v;
  if (DIR* d = ::opendir(p.c_str())) {
    while (dirent* e = ::readdir(d)) {
      std::string n = e->d_name;
      if (n == "." || n == "..") continue;
      std::string ruta = unir(p, n);
      v.push_back({n, esDirectorio(ruta), tamano(ruta)});
    }
    ::closedir(d);
  }
  std::sort(v.begin(), v.end(), [](const Entrada& a, const Entrada& b) { return a.nombre < b.nombre; });
  return v;
}

inline bool borrarTodo(const std::string& p) {
  if (esDirectorio(p)) {
    for (auto& e : listar(p)) borrarTodo(unir(p, e.nombre));
    return std::remove(p.c_str()) == 0;  // remove() también borra carpetas vacías
  }
  return std::remove(p.c_str()) == 0;
}

inline bool renombrar(const std::string& a, const std::string& b) { return std::rename(a.c_str(), b.c_str()) == 0; }

inline bool copiar(const std::string& a, const std::string& b) {
  if (esDirectorio(a)) {
    if (!crearDirectorios(b)) return false;
    bool ok = true;
    for (auto& e : listar(a)) ok = copiar(unir(a, e.nombre), unir(b, e.nombre)) && ok;
    return ok;
  }
  std::string datos;
  return leer(a, datos) && escribir(b, datos);
}

}  // namespace archivos
}  // namespace abla
