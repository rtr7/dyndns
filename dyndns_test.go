package dyndns

import (
	"os/exec"
	"regexp"
	"testing"
)

var re = regexp.MustCompile(`192.0.2.0 via ([^ ]+)`)

func TestDefaultAddr(t *testing.T) {
	out, err := exec.Command("ip", "route", "get", "192.0.2.0").Output()
	if err != nil {
		t.Fatalf("ip route get 192.0.2.0: %v", err)
	}
	matches := re.FindStringSubmatch(string(out))
	if matches == nil {
		t.Fatalf("could not parse ip(8) output")
	}
	want := "http://" + matches[1] + ":8053/dyndns"

	got, err := defaultAddr()
	if err != nil {
		t.Fatal(err)
	}

	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
