package analysis

import "testing"

func TestScopes(t *testing.T) {
	ta := ana.Tables[ana.ByName("singleScope")]
	if _, is := ta.IsFixedSize(); !is {
		t.Fatal()
	}
	if len(ta.Scopes()) != 1 {
		t.Fatal(ta.Scopes())
	}

	l := ana.Tables[ana.ByName("multipleScopes")].Scopes()
	if len(l) != 5 {
		t.Fatal(l)
	}
}
