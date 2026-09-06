package uuidx

import "testing"

func TestV4VersionAndLength(t *testing.T) {
	id := V4()
	if Version(id) != 4 {
		t.Fatalf("v4 version=%d want 4", Version(id))
	}
	if (id[8] & 0xc0) != 0x80 {
		t.Fatalf("v4 variant bits = %02x, want RFC variant", id[8])
	}
}

func TestV7VersionAndLength(t *testing.T) {
	id, err := V7()
	if err != nil {
		t.Fatal(err)
	}
	if Version(id) != 7 {
		t.Fatalf("v7 version=%d want 7", Version(id))
	}
	if (id[8] & 0xc0) != 0x80 {
		t.Fatalf("v7 variant bits = %02x, want RFC variant", id[8])
	}
}

func TestForArm(t *testing.T) {
	v4, err := ForArm("v4")
	if err != nil {
		t.Fatal(err)
	}
	if Version(v4) != 4 {
		t.Fatalf("ForArm(v4) version=%d", Version(v4))
	}
	v7, err := ForArm("v7")
	if err != nil {
		t.Fatal(err)
	}
	if Version(v7) != 7 {
		t.Fatalf("ForArm(v7) version=%d", Version(v7))
	}
	if _, err := ForArm("v1"); err == nil {
		t.Fatal("expected error for unknown arm")
	}
}

func TestV4Unique(t *testing.T) {
	seen := make(map[[16]byte]struct{}, 256)
	for i := 0; i < 256; i++ {
		id := V4()
		if _, ok := seen[id]; ok {
			t.Fatal("duplicate v4 in 256 draws")
		}
		seen[id] = struct{}{}
	}
}
