package provider

import "testing"

// Real catalogue values, copied from the live API.
var tmplLabels = [][]string{
	{"centos-stream-9", "templates/centos-stream-9", "CentOS Stream 9"},
	{"centos-stream-10", "templates/centos-stream-10", "CentOS Stream 10"},
	{"ubuntu-26.04", "templates/ubuntu-26.04", "Ubuntu 26.04 LTS"},
	{"ubuntu-24.04", "templates/ubuntu-24.04", "Ubuntu 24.04 LTS"},
	{"alpine-3.23", "templates/alpine-3.23", "Alpine 3"},
	{"debian-13", "templates/debian-13", "Debian 13"},
	{"rocky-10", "templates/rocky-10", "Rocky 10"},
}

var storageNames = []string{"R1", "W1", "R2", "W2", "R3", "W3", "R4", "W4", "R5", "W5", "E1", "S1"}
var classNames = []string{"bs-burst-xs", "bs-burst-s", "bs-l", "hf-burst-m", "hf-m", "hf-s"}

func TestImageNames(t *testing.T) {
	for _, c := range []struct {
		in   string
		want int
		err  bool
	}{
		{"ubuntu-26.04", 2, false},            // the slug: canonical
		{"templates/ubuntu-26.04", 2, false},  // full api_name
		{"Ubuntu 26.04 LTS", 2, false},        // display name
		{"UBUNTU-26.04", 2, false},            // case does not matter
		{"ubuntu 26", 2, false},               // unique substring of the display name
		{"debian-13", 5, false},
		{"rocky", 6, false},
		{"ubuntu", -1, true},                  // 24.04 and 26.04 both match
		{"centos", -1, true},                  // 9 and 10 both match
		{"windows-2022", -1, true},            // nothing
	} {
		got, err := pickByLabels(c.in, tmplLabels)
		if c.err != (err != nil) || (!c.err && got != c.want) {
			t.Errorf("image %q: got %d err=%v, want %d err=%v", c.in, got, err, c.want, c.err)
		}
	}
}

func TestStorageAndClassCase(t *testing.T) {
	for _, c := range []struct {
		in   string
		want int
	}{
		{"R1", 0}, {"r1", 0}, {" r1 ", 0}, // case and stray spaces
		{"W5", 9}, {"e1", 10},
	} {
		got, err := pickByName(c.in, storageNames)
		if err != nil || got != c.want {
			t.Errorf("storage %q: got %d err=%v, want %d", c.in, got, err, c.want)
		}
	}
	for _, c := range []struct {
		in   string
		want int
	}{
		{"bs-burst-xs", 0}, {"BS-BURST-XS", 0}, {"hf-m", 4},
	} {
		got, err := pickByName(c.in, classNames)
		if err != nil || got != c.want {
			t.Errorf("class %q: got %d err=%v, want %d", c.in, got, err, c.want)
		}
	}
	// "bs-burst" hits three of them - must not silently pick one.
	if _, err := pickByName("bs-burst", classNames); err == nil {
		t.Error("class \"bs-burst\": expected an ambiguity error")
	}
}

func TestStripPrefixLen(t *testing.T) {
	for in, want := range map[string]string{
		"194.110.174.110/24":                  "194.110.174.110",
		"2a01:ea05::62cc:20b7:d4b3:21fc/128":  "2a01:ea05::62cc:20b7:d4b3:21fc",
		"10.58.107.70":                        "10.58.107.70", // already clean
		"":                                    "",
	} {
		if got := stripPrefixLen(in); got != want {
			t.Errorf("stripPrefixLen(%q) = %q, want %q", in, got, want)
		}
	}
}
