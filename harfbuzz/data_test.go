package harfbuzz

import "testing"

func TestLoadFile(t *testing.T) {
	_, err := Files.Open("harfbuzz_reference/in-house/fonts/1a3d8f381387dd29be1e897e4b5100ac8b4829e1.ttf")
	if err != nil {
		t.Fatal(err)
	}
}
