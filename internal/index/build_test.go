package index

import (
	"testing"
)

var sample = [][]byte{
	[]byte("The cat sat on the mat. The CAT!"),
	[]byte("A dog and the cat."),
}

const sampleIndex = "and\t1\tdoc1:1\n" +
	"cat\t3\tdoc0:2,doc1:1\n" +
	"dog\t1\tdoc1:1\n" +
	"mat\t1\tdoc0:1\n" +
	"sat\t1\tdoc0:1\n" +
	"the\t4\tdoc0:3,doc1:1\n"

func TestBuildV0(t *testing.T) {
	build, err := Version("v0")
	if err != nil {
		t.Fatal(err)
	}
	if got := write(t, build(sample, Options{})...); got != sampleIndex {
		t.Errorf("v0 index =\n%s\nwant\n%s", got, sampleIndex)
	}
}

func TestUnknownVersion(t *testing.T) {
	if _, err := Version("v9"); err == nil {
		t.Error("Version(v9) succeeded, want error")
	}
}
