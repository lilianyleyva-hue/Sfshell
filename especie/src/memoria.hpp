#pragma once
// Memoria de un ser.
//
//  - Contexto (memoria de trabajo): los últimos N tokens de recuerdos, en RAM.
//    Por defecto 1.000.000 de tokens por ser.
//  - Largo plazo: TODO lo que el ser ha pensado, en un archivo en disco que
//    solo crece. Nunca olvida; solo deja de tenerlo "presente".
//
// Al despertar, el contexto se reconstruye leyendo la cola del archivo.

#include <cctype>
#include <cstdint>
#include <deque>
#include <filesystem>
#include <fstream>
#include <string>
#include <vector>

namespace abla {

namespace fs = std::filesystem;

class Memoria {
 public:
  struct Recuerdo {
    uint64_t tick;
    std::string texto;
    uint32_t tokens;
  };

  void configurar(uint64_t capacidadTokens, const fs::path& archivo) {
    capacidad_ = capacidadTokens;
    archivo_ = archivo;
    cargar();
    log_.open(archivo_, std::ios::app);
  }

  static uint32_t tokens(const std::string& s) {
    uint32_t n = 0;
    bool enPalabra = false;
    for (unsigned char c : s) {
      bool espacio = c == ' ' || c == '\t' || c == '\n';
      if (!espacio && !enPalabra) n++;
      enPalabra = !espacio;
    }
    return n ? n : 1;
  }

  void recordar(uint64_t tick, const std::string& texto) {
    std::string limpio = texto;
    for (auto& c : limpio)
      if (c == '\n' || c == '\t') c = ' ';
    log_ << tick << '\t' << limpio << '\n';
    totalLargoPlazo_++;
    meter({tick, limpio, tokens(limpio)});
  }

  const std::deque<Recuerdo>& contexto() const { return ventana_; }
  uint64_t usados() const { return usados_; }
  uint64_t capacidad() const { return capacidad_; }
  uint64_t largoPlazo() const { return totalLargoPlazo_; }
  uintmax_t bytesEnDisco() const {
    std::error_code ec;
    auto t = fs::file_size(archivo_, ec);
    return ec ? 0 : t;
  }
  const fs::path& archivo() const { return archivo_; }
  void sincronizar() { log_.flush(); }

  // Busca primero en el contexto y luego en todo el largo plazo.
  std::vector<std::string> buscar(const std::string& consulta, size_t max) {
    auto bajo = [](std::string x) {
      for (auto& c : x) c = static_cast<char>(std::tolower(static_cast<unsigned char>(c)));
      return x;
    };
    const std::string q = bajo(consulta);
    std::vector<std::string> r;
    for (auto it = ventana_.rbegin(); it != ventana_.rend() && r.size() < max; ++it)
      if (bajo(it->texto).find(q) != std::string::npos)
        r.push_back("[t=" + std::to_string(it->tick) + "] " + it->texto);
    if (r.size() < max && totalLargoPlazo_ > ventana_.size()) {
      log_.flush();
      std::ifstream in(archivo_);
      std::string linea;
      uint64_t enDisco = 0, limite = totalLargoPlazo_ - ventana_.size();
      while (std::getline(in, linea) && enDisco++ < limite && r.size() < max) {
        auto tab = linea.find('\t');
        if (tab != std::string::npos && bajo(linea).find(q, tab) != std::string::npos)
          r.push_back("[t=" + linea.substr(0, tab) + ", largo plazo] " + linea.substr(tab + 1));
      }
    }
    return r;
  }

 private:
  void meter(Recuerdo r) {
    usados_ += r.tokens;
    ventana_.push_back(std::move(r));
    while (usados_ > capacidad_ && ventana_.size() > 1) {
      usados_ -= ventana_.front().tokens;
      ventana_.pop_front();
    }
  }

  void cargar() {
    std::ifstream in(archivo_);
    std::string linea;
    while (std::getline(in, linea)) {
      auto tab = linea.find('\t');
      if (tab == std::string::npos) continue;
      totalLargoPlazo_++;
      std::string texto = linea.substr(tab + 1);
      meter({std::stoull(linea.substr(0, tab)), texto, tokens(texto)});
    }
  }

  uint64_t capacidad_ = 1'000'000, usados_ = 0, totalLargoPlazo_ = 0;
  std::deque<Recuerdo> ventana_;
  fs::path archivo_;
  std::ofstream log_;
};

}  // namespace abla
