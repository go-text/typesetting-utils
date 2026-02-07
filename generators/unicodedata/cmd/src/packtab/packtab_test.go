package packtab

import (
	"fmt"
	"testing"
)

func TestTables(t *testing.T) {
	sol := PackTable([]int{
		12, 12, 12, 12, 15, 15, 15, 15,
		3, 3, 3, 3, 3, 3, 3, 3,
		-4, -4, -4, -4, -4, -4, -4, -4,
		-4, -4, -4, -4, -4, -4, -4, -4,
	}, 2, 9)

	fmt.Println(sol.Code("gc"))
}
