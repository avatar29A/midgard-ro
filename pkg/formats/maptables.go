package formats

import (
	"bufio"
	"bytes"
	"strconv"
	"strings"
)

// The client's per-map camera rules live in two text tables in the archive,
// CP949 with Korean comments. The entries themselves are ASCII, one per line,
// '#'-terminated, so the tables are read byte-wise and the comments — which
// start with "//" — are simply dropped.

// ParseIndoorRSWTable parses data/indoorrswtable.txt: one "name.rsw#" per
// line. A map listed here is indoor, and the original client disables orbital
// camera rotation on it. Keys are lower-case map names without the
// extension, so "prt_in.rsw#" is looked up as "prt_in".
func ParseIndoorRSWTable(data []byte) map[string]bool {
	indoor := make(map[string]bool)
	for _, line := range tableLines(data) {
		for _, field := range strings.Split(line, "#") {
			if name := mapTableKey(field); name != "" {
				indoor[name] = true
			}
		}
	}
	return indoor
}

// ViewPoint is one entry of data/viewpointtable.txt: a map's camera preset.
// The table's own header names the fields and the normal-map defaults —
// "range 230, scope 170", "rotationFrom -360, To 360", "altitudeFrom -50,
// To -65" — with the *In fields being the values on entering the map.
type ViewPoint struct {
	Range, Scope, RangeIn                float32
	RotationFrom, RotationTo, RotationIn float32
	AltitudeFrom, AltitudeTo, AltitudeIn float32
}

// Unrestricted reports whether the map lets the camera orbit the whole way
// round, which the table spells as -360 to 360.
func (v ViewPoint) Unrestricted() bool {
	return v.RotationFrom <= -360 && v.RotationTo >= 360
}

// Fixed reports whether the map allows no rotation at all: from and to are
// the same angle.
func (v ViewPoint) Fixed() bool {
	return v.RotationFrom == v.RotationTo
}

// viewPointFields is how many '#'-separated values follow the map name.
const viewPointFields = 9

// ParseViewpointTable parses data/viewpointtable.txt. Lines that do not
// carry a name and nine numbers are skipped rather than failing the table:
// the file has typos in it (a stray "/" on one line) and one bad preset
// should not cost every other map its rules.
func ParseViewpointTable(data []byte) map[string]ViewPoint {
	presets := make(map[string]ViewPoint)
	for _, line := range tableLines(data) {
		// A lone slash appears in one entry of the real file, between two
		// numbers. It is not part of any value.
		line = strings.ReplaceAll(line, "/", "")

		fields := strings.Split(line, "#")
		if len(fields) < viewPointFields+1 {
			continue
		}
		name := mapTableKey(fields[0])
		if name == "" {
			continue
		}

		var values [viewPointFields]float32
		ok := true
		for i := range values {
			f, err := strconv.ParseFloat(strings.TrimSpace(fields[i+1]), 32)
			if err != nil {
				ok = false
				break
			}
			values[i] = float32(f)
		}
		if !ok {
			continue
		}

		presets[name] = ViewPoint{
			Range: values[0], Scope: values[1], RangeIn: values[2],
			RotationFrom: values[3], RotationTo: values[4], RotationIn: values[5],
			AltitudeFrom: values[6], AltitudeTo: values[7], AltitudeIn: values[8],
		}
	}
	return presets
}

// tableLines splits a table into lines with comments and blank lines gone.
func tableLines(data []byte) []string {
	var lines []string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// mapTableKey normalizes a map file name from a table into the key the rest
// of the client uses: lower case, no extension, no surrounding space. Empty
// for a field that is not a name.
func mapTableKey(field string) string {
	name := strings.ToLower(strings.TrimSpace(field))
	name = strings.TrimSuffix(name, ".rsw")
	name = strings.TrimSuffix(name, ".gat")
	if name == "" || strings.ContainsAny(name, " \t") {
		return ""
	}
	return name
}
