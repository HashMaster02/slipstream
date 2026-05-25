package main

import (
	"fmt"

	"github.com/HashMaster02/slipstream/pkg/types"
)

func main() {

	var val float64 = 130.0
	var price types.Price = types.PriceFromFloat(val)
	fmt.Printf("%s\n", price.String())
	
}