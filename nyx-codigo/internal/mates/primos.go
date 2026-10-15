package mates

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"time"

	"nyxcodigo/internal/nucleo"
)

// Factor is a prime power Primo^Exp.
type Factor struct {
	Primo *big.Int
	Exp   int
}

const (
	limiteTanteo       = 1_000_000
	tiempoFactorizar   = 3 * time.Second
	maxDivisores       = 10_000
	rondasMillerRabin  = 20
	tiempoDivisoresMax = 3 * time.Second
	// maxBitsPrimalidad bounds the numbers given to Miller-Rabin inside Factorizar and the resolvers:
	// 20 rounds on 4096 bits take about a second; on 16 384 bits, half a minute.
	maxBitsPrimalidad = 4_096
	// pasoCtx: the trial-division loop looks at the deadline every this many candidate divisors.
	pasoCtx = 2_048
)

// ErrPrimalidadGrande: the number is too big for Miller-Rabin to answer in reasonable time.
var ErrPrimalidadGrande = fmt.Errorf("%w: solo sé si un número es primo hasta %d cifras", nucleo.ErrNoSoportado, maxCifrasPrimalidad)

// maxCifrasPrimalidad is maxBitsPrimalidad in decimal digits (bits·log10 2).
const maxCifrasPrimalidad = maxBitsPrimalidad * 30103 / 100000

// EsPrimo: Miller-Rabin with 20 rounds (plus Baillie-PSW, as big.Int.ProbablyPrime does). It has no
// size limit: callers that take user input bound the size first (cabeEnPrimalidad).
func EsPrimo(x *big.Int) bool {
	return x != nil && x.Sign() > 0 && x.ProbablyPrime(rondasMillerRabin)
}

// cabeEnPrimalidad reports whether Miller-Rabin on x is affordable.
func cabeEnPrimalidad(x *big.Int) bool { return x.BitLen() <= maxBitsPrimalidad }

// MCD is the greatest common divisor of |a| and |b| (mcd(0, 0) = 0).
func MCD(a, b *big.Int) *big.Int {
	return new(big.Int).GCD(nil, nil, new(big.Int).Abs(a), new(big.Int).Abs(b))
}

// MCM is the least common multiple of |a| and |b| (0 if either is 0).
func MCM(a, b *big.Int) *big.Int {
	if a.Sign() == 0 || b.Sign() == 0 {
		return new(big.Int)
	}
	return mcmInt(new(big.Int).Abs(a), new(big.Int).Abs(b))
}

// Factorizar returns the prime factorization of |x| (x ≠ 0): trial division up to 10⁶, then
// Pollard-Brent for what is left, with a 3 s limit (or ctx, whichever ends first). The deadline is
// checked throughout; Miller-Rabin only runs on cofactors of at most maxBitsPrimalidad bits, so a
// huge number that trial division does not finish gives an honest error instead of a long wait.
func Factorizar(ctx context.Context, x *big.Int) ([]Factor, error) {
	if x == nil || x.Sign() == 0 {
		return nil, errors.New("el 0 no se puede descomponer en factores primos")
	}
	ctx, cancel := context.WithTimeout(ctx, tiempoFactorizar)
	defer cancel()
	n := new(big.Int).Abs(x)
	cuenta := map[string]int{}
	primos := map[string]*big.Int{}
	añadir := func(p *big.Int, k int) {
		s := p.String()
		if _, ok := primos[s]; !ok {
			primos[s] = new(big.Int).Set(p)
		}
		cuenta[s] += k
	}
	if cabeEnPrimalidad(n) && EsPrimo(n) {
		return []Factor{{Primo: n, Exp: 1}}, nil
	}
	// trial division
	var q, r big.Int
	d := new(big.Int)
	vueltas := 0
	for i := int64(2); i <= limiteTanteo; {
		if n.Cmp(big.NewInt(1)) == 0 {
			break
		}
		d.SetInt64(i)
		if new(big.Int).Mul(d, d).Cmp(n) > 0 {
			break
		}
		k := 0
		for {
			q.QuoRem(n, d, &r)
			if r.Sign() != 0 {
				break
			}
			n.Set(&q)
			k++
		}
		if k > 0 {
			añadir(d, k)
			// a prime cofactor ends the search early (only when the test is cheap)
			if cabeEnPrimalidad(n) && EsPrimo(n) {
				break
			}
		}
		vueltas++
		if vueltas%pasoCtx == 0 && ctx.Err() != nil {
			return nil, fmt.Errorf("%w: no he terminado de descomponer %s", nucleo.ErrSinTiempo, recortar(x.String()))
		}
		if i == 2 {
			i = 3
		} else {
			i += 2
		}
	}
	if n.Cmp(big.NewInt(1)) > 0 {
		pila := []*big.Int{n}
		for len(pila) > 0 {
			if ctx.Err() != nil {
				return nil, fmt.Errorf("%w: no he terminado de descomponer %s", nucleo.ErrSinTiempo, recortar(x.String()))
			}
			m := pila[len(pila)-1]
			pila = pila[:len(pila)-1]
			if m.Cmp(big.NewInt(1)) == 0 {
				continue
			}
			if !cabeEnPrimalidad(m) {
				return nil, fmt.Errorf("%w: queda un factor de %d cifras y no sé si es primo", nucleo.ErrNoSoportado, len(m.String()))
			}
			if EsPrimo(m) {
				añadir(m, 1)
				continue
			}
			// perfect powers first (Pollard struggles with p^k)
			if b, k, ok := potenciaPerfecta(ctx, m); ok {
				for i := 0; i < k; i++ {
					pila = append(pila, new(big.Int).Set(b))
				}
				continue
			}
			f, err := pollardBrent(ctx, m)
			if err != nil {
				return nil, err
			}
			pila = append(pila, f, new(big.Int).Quo(m, f))
		}
	}
	out := make([]Factor, 0, len(primos))
	for s, p := range primos {
		out = append(out, Factor{Primo: p, Exp: cuenta[s]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Primo.Cmp(out[j].Primo) < 0 })
	return out, nil
}

// potenciaPerfecta finds m = b^k with k ≥ 2. m has no prime factor below 10⁶ (trial division ran
// first), so b ≥ 10⁶ > 2^19 and k ≤ bits/19.
func potenciaPerfecta(ctx context.Context, m *big.Int) (*big.Int, int, bool) {
	for k := m.BitLen() / 19; k >= 2; k-- {
		if k%64 == 0 && ctx.Err() != nil {
			return nil, 0, false
		}
		if b, ok := raizEntera(m, int64(k)); ok && b.Cmp(big.NewInt(1)) > 0 {
			return b, k, true
		}
	}
	return nil, 0, false
}

// pollardBrent finds a nontrivial factor of the composite n (Brent's variant of Pollard's rho).
func pollardBrent(ctx context.Context, n *big.Int) (*big.Int, error) {
	if n.Bit(0) == 0 {
		return big.NewInt(2), nil
	}
	uno := big.NewInt(1)
	for c := int64(1); c < 1000; c++ {
		cc := big.NewInt(c)
		f := func(x *big.Int) *big.Int {
			r := new(big.Int).Mul(x, x)
			r.Add(r, cc)
			return r.Mod(r, n)
		}
		y := big.NewInt(2)
		var x, ys *big.Int
		g := big.NewInt(1)
		q := big.NewInt(1)
		const m = 128
		for rr := 1; g.Cmp(uno) == 0; rr *= 2 {
			x = new(big.Int).Set(y)
			for i := 0; i < rr; i++ {
				if i%m == 0 && ctx.Err() != nil {
					return nil, fmt.Errorf("%w: no he podido descomponer %s a tiempo", nucleo.ErrSinTiempo, recortar(n.String()))
				}
				y = f(y)
			}
			for k := 0; k < rr && g.Cmp(uno) == 0; k += m {
				if ctx.Err() != nil {
					return nil, fmt.Errorf("%w: no he podido descomponer %s a tiempo", nucleo.ErrSinTiempo, recortar(n.String()))
				}
				ys = new(big.Int).Set(y)
				for i := 0; i < min(m, rr-k); i++ {
					y = f(y)
					d := new(big.Int).Sub(x, y)
					d.Abs(d)
					q.Mul(q, d)
					q.Mod(q, n)
				}
				g = new(big.Int).GCD(nil, nil, q, n)
			}
			if rr > 1<<26 {
				break
			}
		}
		if g.Cmp(n) == 0 {
			// backtrack
			for i := 0; ; i++ {
				if i%m == 0 && ctx.Err() != nil {
					return nil, fmt.Errorf("%w: no he podido descomponer %s a tiempo", nucleo.ErrSinTiempo, recortar(n.String()))
				}
				ys = f(ys)
				d := new(big.Int).Sub(x, ys)
				d.Abs(d)
				g = new(big.Int).GCD(nil, nil, d, n)
				if g.Cmp(uno) > 0 {
					break
				}
			}
		}
		if g.Cmp(uno) > 0 && g.Cmp(n) < 0 {
			return g, nil
		}
	}
	return nil, fmt.Errorf("%w: no encuentro factores de %s", nucleo.ErrNoSoportado, recortar(n.String()))
}

// Divisores returns the positive divisors of |x| in order (at most 10 000 divisors).
func Divisores(x *big.Int) ([]*big.Int, error) {
	if x == nil || x.Sign() == 0 {
		return nil, errors.New("el 0 tiene infinitos divisores")
	}
	ctx, cancel := context.WithTimeout(context.Background(), tiempoDivisoresMax)
	defer cancel()
	fs, err := Factorizar(ctx, x)
	if err != nil {
		return nil, err
	}
	total := 1
	for _, f := range fs {
		total *= f.Exp + 1
		if total > maxDivisores {
			return nil, fmt.Errorf("tiene más de %d divisores", maxDivisores)
		}
	}
	out := []*big.Int{big.NewInt(1)}
	for _, f := range fs {
		var nuevos []*big.Int
		p := big.NewInt(1)
		for k := 1; k <= f.Exp; k++ {
			p = new(big.Int).Mul(p, f.Primo)
			for _, d := range out {
				nuevos = append(nuevos, new(big.Int).Mul(d, p))
			}
		}
		out = append(out, nuevos...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Cmp(out[j]) < 0 })
	return out, nil
}

// divisorPequeño looks for a divisor of x among 2 and the odd numbers up to limite (and up to √x):
// the cheap check that decides "no es primo" for most big composites.
func divisorPequeño(ctx context.Context, x *big.Int, limite int64) (*big.Int, bool) {
	var q, r big.Int
	d := new(big.Int)
	vueltas := 0
	for i := int64(2); i <= limite; {
		d.SetInt64(i)
		if new(big.Int).Mul(d, d).Cmp(x) > 0 {
			break
		}
		if q.QuoRem(x, d, &r); r.Sign() == 0 {
			return new(big.Int).Set(d), true
		}
		vueltas++
		if vueltas%pasoCtx == 0 && ctx.Err() != nil {
			break
		}
		if i == 2 {
			i = 3
		} else {
			i += 2
		}
	}
	return nil, false
}

// tanteoCompleto divides x (at most 40 bits) by every number from 2 to √x and reports the first
// divisor found, so the trace can say the check was really done.
func tanteoCompleto(x uint64) (uint64, bool) {
	if x < 4 {
		return 0, false
	}
	if x%2 == 0 {
		return 2, true
	}
	for d := uint64(3); d*d <= x; d += 2 {
		if x%d == 0 {
			return d, true
		}
	}
	return 0, false
}

// textoFactores writes "2^3 · 3 · 5^2".
func textoFactores(fs []Factor) string {
	s := ""
	for i, f := range fs {
		if i > 0 {
			s += " · "
		}
		s += f.Primo.String()
		if f.Exp > 1 {
			s += "^" + fmt.Sprint(f.Exp)
		}
	}
	return s
}

// producto multiplies the factors back.
func producto(fs []Factor) *big.Int {
	r := big.NewInt(1)
	for _, f := range fs {
		r.Mul(r, new(big.Int).Exp(f.Primo, big.NewInt(int64(f.Exp)), nil))
	}
	return r
}
