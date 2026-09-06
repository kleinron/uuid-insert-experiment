package config

import "testing"

func TestPasswordFromSecretStringJSON(t *testing.T) {
	pw, err := passwordFromSecretString(`{"username":"exp_app","password":"s3cret"}`)
	if err != nil {
		t.Fatal(err)
	}
	if pw != "s3cret" {
		t.Fatalf("got %q", pw)
	}
}

func TestPasswordFromSecretStringPlain(t *testing.T) {
	pw, err := passwordFromSecretString("plain-password")
	if err != nil {
		t.Fatal(err)
	}
	if pw != "plain-password" {
		t.Fatalf("got %q", pw)
	}
}

func TestPasswordFromSecretStringMissing(t *testing.T) {
	if _, err := passwordFromSecretString(`{"username":"exp_app"}`); err == nil {
		t.Fatal("expected error")
	}
}
