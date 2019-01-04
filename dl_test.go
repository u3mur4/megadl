package megadl_test

import (
	"bytes"
	"io/ioutil"
	"path"
	"testing"

	"github.com/u3mur4/megadl"
)

func TestDownload(t *testing.T) {
	var filetest = []struct {
		url      string
		filename string
		filesize int
	}{
		{"https://mega.nz/#!0rASQYSR!KD1y_pMnRAJkgp1sPtcno5L548L1WJcfQhN0SCITuI4", "random.bin", 100000},
		{"https://mega.nz/#!gqpwnaAD!VuAFRnqtVV8KSnXi1FKIjmIeYoe9owHrLpaXUDhLP2o", "random.txt", 100000},
	}

	for _, ft := range filetest {
		rc, info, err := megadl.Download(ft.url)

		if err != nil {
			t.Errorf("cannot start download: %v\n", err)
			continue
		}

		if info.Name != ft.filename {
			t.Errorf("got filename %s, expected: %s", info.Name, ft.filename)
		}

		if info.Size != ft.filesize {
			t.Errorf("got filesize %d, expected: %d", info.Size, ft.filesize)
		}

		d, err := ioutil.ReadAll(rc)
		if err != nil {
			t.Errorf("cannot download %s: %v", info.Name, err)
			continue
		}

		o, err := ioutil.ReadFile(path.Join("testdata", ft.filename))
		if err != nil {
			t.Errorf("cannot open test file %s: %v\n", "random.bin", err)
			continue
		}

		r := bytes.Compare(d, o)
		if r != 0 {
			t.Errorf("corrupted download")
		}
	}
}
