package i18n

import "testing"

func TestL(t *testing.T) {
	defer Set(ID)
	if L("Halo", "Hello") != "Halo" {
		t.Fatal("default must be Indonesian")
	}
	Set(EN)
	if L("Halo", "Hello") != "Hello" || F("%d berkas", "%d files", 3) != "3 files" {
		t.Fatal("English not applied")
	}
	Set("fr")
	if Lang() != ID {
		t.Fatal("unknown language must fall back to Indonesian")
	}
}
