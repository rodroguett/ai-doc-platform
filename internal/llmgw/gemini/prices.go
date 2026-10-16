package gemini

// price es la tarifa de lista de un modelo en la capa de pago, en dólares por
// millón de tokens.
type price struct {
	input, output float64
}

// prices son las tarifas con que se estima el costo de cada llamada
// (ADR-0013). El proyecto corre en la capa gratuita y no paga nada: estas
// tarifas dicen cuánto habría costado.
//
// Tomadas de https://ai.google.dev/gemini-api/docs/pricing el 2026-10-15. La
// de gemini-embedding-001 ya no figura allí; es la del anuncio de su
// disponibilidad general, del 14 de julio de 2025. Cambiar una tarifa, o
// agregar un modelo, es un cambio a esta tabla.
var prices = map[string]price{
	ModelFlashLite:         {input: 0.30, output: 2.50},
	ModelFlashLiteFallback: {input: 0.25, output: 1.50},
	model:                  {input: 0.15},
}

// estimateCost devuelve el costo estimado de una llamada, o nil si el modelo
// no tiene tarifa conocida.
func estimateCost(model string, inputTokens, outputTokens int) *float64 {
	p, ok := prices[model]
	if !ok {
		return nil
	}
	cost := (float64(inputTokens)*p.input + float64(outputTokens)*p.output) / 1e6
	return &cost
}
