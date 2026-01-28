package wsl

import "testing"

func TestParseDistroList(t *testing.T) {
	input := `  NAME            STATE           VERSION
* Ubuntu-22.04    Running         2
  Debian          Stopped         2
`
	distros, err := ParseDistroList(input)
	if err != nil {
		t.Fatalf("ParseDistroList: %v", err)
	}
	if len(distros) != 2 {
		t.Fatalf("expected 2 distros, got %d", len(distros))
	}
	if distros[0].Name != "Ubuntu-22.04" || !distros[0].Default || distros[0].Version != 2 {
		t.Fatalf("unexpected distro[0]: %+v", distros[0])
	}
	if distros[1].Name != "Debian" || distros[1].Default {
		t.Fatalf("unexpected distro[1]: %+v", distros[1])
	}
}

func TestParseDistroListNoRows(t *testing.T) {
	_, err := ParseDistroList("NAME STATE VERSION\n")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestSelectDistroPreferred(t *testing.T) {
	distros := []Distro{{Name: "Ubuntu"}, {Name: "Debian"}}
	got, err := SelectDistro(distros, "Debian")
	if err != nil {
		t.Fatalf("SelectDistro: %v", err)
	}
	if got != "Debian" {
		t.Fatalf("expected Debian, got %q", got)
	}
}

func TestSelectDistroPreferredMissing(t *testing.T) {
	distros := []Distro{{Name: "Ubuntu"}}
	if _, err := SelectDistro(distros, "Debian"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestSelectDistroDefault(t *testing.T) {
	distros := []Distro{{Name: "Ubuntu", Default: true}, {Name: "Debian"}}
	got, err := SelectDistro(distros, "")
	if err != nil {
		t.Fatalf("SelectDistro: %v", err)
	}
	if got != "Ubuntu" {
		t.Fatalf("expected Ubuntu, got %q", got)
	}
}

func TestSelectDistroSingle(t *testing.T) {
	distros := []Distro{{Name: "Ubuntu"}}
	got, err := SelectDistro(distros, "")
	if err != nil {
		t.Fatalf("SelectDistro: %v", err)
	}
	if got != "Ubuntu" {
		t.Fatalf("expected Ubuntu, got %q", got)
	}
}

func TestSelectDistroAmbiguous(t *testing.T) {
	distros := []Distro{{Name: "Ubuntu"}, {Name: "Debian"}}
	if _, err := SelectDistro(distros, ""); err == nil {
		t.Fatalf("expected error")
	}
}
