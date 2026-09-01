package dimension_test

import (
	"fmt"

	"github.com/timzifer/metrology/dimension"
)

func ExampleNew() {
	pressure := dimension.New(dimension.Exponents{Length: -1, Mass: 1, Time: -2})
	fmt.Println(pressure)
	// Output: L⁻¹M¹T⁻²
}

func ExampleQuotient() {
	force := dimension.New(dimension.Exponents{Length: 1, Mass: 1, Time: -2})
	area := dimension.L.Pow(2)
	fmt.Println(dimension.Quotient(force, area))
	// Output: L⁻¹M¹T⁻²
}

func ExampleProduct() {
	force := dimension.New(dimension.Exponents{Length: 1, Mass: 1, Time: -2})
	fmt.Println(dimension.Product(force, dimension.L))
	// Output: L²M¹T⁻²
}

func ExampleDimension_Reciprocal() {
	frequency := dimension.T.Reciprocal()
	fmt.Println(frequency, dimension.Product(frequency, dimension.T))
	// Output: T⁻¹ 1
}
