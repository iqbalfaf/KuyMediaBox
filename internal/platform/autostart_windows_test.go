package platform

import "testing"

func TestAutostart(t *testing.T) {
	const name = "KuyMediaBoxTest"
	cmd := `"C:\Program Files\KuyMediaBox\KuyMediaBox.exe" --tray`
	if err := SetAutostart(name, cmd, true); err != nil {
		t.Fatal(err)
	}
	defer SetAutostart(name, "", false)
	if got := AutostartCommand(name); got != cmd {
		t.Fatalf("command = %q", got)
	}
	if err := SetAutostart(name, "", false); err != nil {
		t.Fatal(err)
	}
	if got := AutostartCommand(name); got != "" {
		t.Fatalf("still registered: %q", got)
	}
	if err := SetAutostart(name, "", false); err != nil { // removing twice is fine
		t.Fatal(err)
	}
}
