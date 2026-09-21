package packs

import "testing"

func TestStarterPacksParse(t *testing.T) {
	for _, name := range Names {
		raw, err := Read(name)
		if err != nil {
			t.Fatal(name, err)
		}
		f, err := Parse(raw)
		if err != nil {
			t.Fatal(name, err)
		}
		if f.Name != name {
			t.Fatalf("%s named %s", name, f.Name)
		}
	}
}
