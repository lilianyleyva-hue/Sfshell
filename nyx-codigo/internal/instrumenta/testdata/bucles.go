package main

import "fmt"

var contador int

func Factorial(n int) int {
	if n <= 1 {
		return 1
	}
	return n * Factorial(n-1)
}

func esPar(n int) bool {
	if n == 0 {
		return true
	}
	return esImpar(n - 1)
}

func esImpar(n int) bool {
	if n == 0 {
		return false
	}
	return esPar(n - 1)
}

func Cuenta(xs []int) (pares int) {
	for i := 0; i < len(xs); i++ {
		if esPar(xs[i]) {
			pares++
		}
	}
	return
}

func Busca(m [][]int, x int) (int, int) {
fuera:
	for i, fila := range m {
		for j := range fila {
			if fila[j] == x {
				return i, j
			}
			if fila[j] > x {
				continue fuera
			}
		}
	}
	return -1, -1
}

func Salta(n int) int {
	i := 0
otra:
	i++
	if i < n {
		goto otra
	}
	return i
}

func main() {
	var fib func(int) int
	fib = func(n int) int {
		if n < 2 {
			return n
		}
		return fib(n-1) + fib(n-2)
	}
	for {
		contador++
		if contador > 3 { break }
	}
	if x := fib(10); x > 50 {
		fmt.Println("grande", x)
	} else {
		fmt.Println("pequeño", x)
	}
	switch y := Factorial(4); {
	case y > 20:
		fmt.Println(y)
	}
	var a, b = 1, 2
	a, b = b, a
	fmt.Println(a, b, Cuenta([]int{1, 2, 3, 4}), Salta(3))
	fmt.Println(Busca([][]int{{1, 2}, {3, 4}}, 3))
}
