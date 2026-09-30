// Package service contiene los tipos de dominio del gateway y las
// implementaciones que responden a las operaciones de la API.
//
// Este paquete no conoce HTTP ni los tipos generados desde el contrato: los
// handlers traducen entre ambos mundos. Así el contrato puede cambiar sin
// tocar la lógica, y la lógica sin tocar el contrato.
package service

import "errors"

// ErrInvalidInput indica que la entrada no cumple las reglas del dominio.
// Los errores que lo envuelven llevan un mensaje apto para el cliente.
var ErrInvalidInput = errors.New("entrada inválida")
