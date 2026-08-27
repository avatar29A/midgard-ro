package formats

import "testing"

// A slice of the real indoorrswtable.txt: CP949 comments (here as raw
// bytes), CRLF line ends, one name per line.
var indoorSample = []byte("//20211105 EP20 \xbe\xf3\xc0\xbd\xbc\xba \xb3\xbb\xba\xce\r\n" +
	"icas_in2.rsw#\r\n" +
	"\r\n" +
	"//20220628\r\n" +
	"prt_in.rsw#\r\n" +
	"PRT_CASTLE.rsw#\r\n")

func TestParseIndoorRSWTable(t *testing.T) {
	indoor := ParseIndoorRSWTable(indoorSample)

	for _, name := range []string{"icas_in2", "prt_in", "prt_castle"} {
		if !indoor[name] {
			t.Errorf("%s missing from the indoor table", name)
		}
	}
	if indoor["prontera"] {
		t.Error("prontera is not indoor")
	}
	if len(indoor) != 3 {
		t.Errorf("%d entries, want 3: %v", len(indoor), indoor)
	}
}

// A slice of the real viewpointtable.txt, including its header comments,
// its mixed whitespace and the stray "/" on the job_star line.
var viewpointSample = []byte("// range 230\xbf\xa1 scope 170\xc0\xcc \xba\xb8\xc5\xeb\xb8\xca \xbc\xb3\xc1\xa4\r\n" +
	"// \xb8\xca \xc0\xcc\xb8\xa7 \trange\tscope\r\n" +
	"1@infi.rsw#            230#\t170#\t500#\t\t  0#\t\t  0#\t            -65#\t\t-50#                     -65#                    -65#\t\t\r\n" +
	"mosk_fild01.rsw#\t200#\t70#\t220#\t\t30#\t\t120#\t\t0#\t\t-50#\t\t-65#\t\t-50#\r\n" +
	"job_star.rsw#\t400#\t0#\t400#\t\t0#\t\t0#\t/\t0#\t\t-20#\t\t-20#\t\t-20#\r\n" +
	"veins.rsw#\t350#\t200#\t400#\t\t-360#\t\t360#\t\t0#\t\t-28#\t\t-65#\t\t-50#\r\n" +
	"broken.rsw#\t1#\t2#\r\n")

func TestParseViewpointTable(t *testing.T) {
	presets := ParseViewpointTable(viewpointSample)

	tests := []struct {
		name         string
		want         ViewPoint
		fixed, whole bool
	}{
		{"1@infi", ViewPoint{230, 170, 500, 0, 0, -65, -50, -65, -65}, true, false},
		{"mosk_fild01", ViewPoint{200, 70, 220, 30, 120, 0, -50, -65, -50}, false, false},
		{"job_star", ViewPoint{400, 0, 400, 0, 0, 0, -20, -20, -20}, true, false},
		{"veins", ViewPoint{350, 200, 400, -360, 360, 0, -28, -65, -50}, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := presets[tt.name]
			if !ok {
				t.Fatalf("%s missing: %v", tt.name, presets)
			}
			if got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
			if got.Fixed() != tt.fixed || got.Unrestricted() != tt.whole {
				t.Fatalf("Fixed=%v Unrestricted=%v, want %v %v", got.Fixed(), got.Unrestricted(), tt.fixed, tt.whole)
			}
		})
	}

	if _, ok := presets["broken"]; ok {
		t.Fatal("a line with too few fields was accepted")
	}
	if len(presets) != 4 {
		t.Fatalf("%d presets, want 4", len(presets))
	}
}
