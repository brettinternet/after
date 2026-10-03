package compare

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"testing/quick"
)

func TestJSONGolden(t *testing.T) {
	a := `{"z":null,"n":9007199254740992,"a":[1,2],"/~":"é","gone":true,"eq":1e3}`
	b := `{"a":[2,1,3],"n":9007199254740993,"new":null,"/~":"é","eq":1000.0}`
	got, err := JSON([]byte(a), []byte(b))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.MarshalIndent(got, "", "  ")
	raw = append(raw, '\n')
	want, err := os.ReadFile("testdata/differences.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != string(want) {
		t.Fatalf("golden mismatch:\n%s", raw)
	}
}
func TestJSONExactAndInvalid(t *testing.T) {
	for _, pair := range [][2]string{{`1`, `1.00`}, {`-0`, `0e1000000`}, {`123e999999`, `1230e999998`}, {`{"β":1,"a":2}`, `{"a":2,"β":1}`}, {`"\ud83d\ude00"`, `"😀"`}} {
		got, err := JSON([]byte(pair[0]), []byte(pair[1]))
		if err != nil || len(got) != 0 {
			t.Fatalf("%v: %v %v", pair, got, err)
		}
	}
	for _, bad := range []string{`{"a":1,"a":2}`, `{"a":1,"\u0061":2}`, `"\ud800"`, `"\udc00"`, `1 2`, `[`, "\"\xff\"", `1e999999999999999999`, strings.Repeat("[", 66) + "0" + strings.Repeat("]", 66), strings.Repeat(" ", maxJSON+1)} {
		if _, err := JSON([]byte(bad), []byte(bad)); err == nil {
			t.Fatalf("accepted ambiguous/budget input %.80s", bad)
		}
	}
	a := "[" + strings.Repeat("0,", maxChanges) + "0]"
	b := strings.ReplaceAll(a, "0", "1")
	if _, err := JSON([]byte(a), []byte(b)); err == nil {
		t.Fatal("unbounded witnesses")
	}
}
func TestJSONProperties(t *testing.T) {
	if err := quick.Check(func(n int64, s string) bool {
		text, _ := json.Marshal(s)
		a := []byte(fmt.Sprintf(`{"s":%s,"n":%d}`, text, n))
		b := []byte(fmt.Sprintf(`{"n":%d.0,"s":%s}`, n, text))
		diff, err := JSON(a, b)
		return err == nil && len(diff) == 0
	}, &quick.Config{MaxCount: 500}); err != nil {
		t.Fatal(err)
	}
}
func FuzzJSON(f *testing.F) {
	for _, s := range []string{`null`, `{"a":1,"b":["😀",9007199254740993]}`, `{"a":0,"a":1}`, `"\ud800"`} {
		f.Add(s, `{"added":null}`)
	}
	f.Fuzz(func(t *testing.T, a, b string) {
		x, e1 := JSON([]byte(a), []byte(b))
		y, e2 := JSON([]byte(a), []byte(b))
		if (e1 == nil) != (e2 == nil) || !reflect.DeepEqual(x, y) {
			t.Fatal("nondeterministic")
		}
		if e1 == nil {
			self, e := JSON([]byte(a), []byte(a))
			if e != nil || len(self) != 0 {
				t.Fatal("not reflexive")
			}
		}
	})
}
