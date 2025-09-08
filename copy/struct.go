package copy

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type StringifyRule struct {
	TimeFormat     string
	IfZeroUseEmpty bool
}

type ObjectCopyOptions struct {
	Stringify         map[string]StringifyRule
	UseJSONTags       bool
	DefaultTimeFormat string

	// Null handling
	PgtypeNullStringAsEmpty bool
	PgtypeNullTimeAsEmpty   bool
	PgtypeNullBoolAsFalse   bool
	PgtypeNullIntAsZero     bool
	PgtypeNullFloatAsZero   bool
	PgtypeNullNumericAsZero bool

	// Konversi preferensi
	NumericAsString  bool   // true=string "123.45"; false=float64 (bila bisa)
	JSONAsRawMessage bool   // true=[]byte (json.RawMessage); false=decode ke interface{}
	UUIDAsString     bool   // true="xxxxxxxx-xxxx-...."
	InetAsString     bool   // true="1.2.3.4/24"
	ByteaEncoding    string // "base64" (default) atau "hex"
}

var timeType = reflect.TypeOf(time.Time{})
var timePtrType = reflect.TypeOf(&time.Time{})

// === Core transformer ===

func Struct(src any, dst any, opts ObjectCopyOptions) error {
	if opts.DefaultTimeFormat == "" {
		opts.DefaultTimeFormat = time.RFC3339
	}
	transformed := transformValue(reflect.ValueOf(src), "", &opts)
	b, err := json.Marshal(transformed)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}

func transformValue(v reflect.Value, path string, opts *ObjectCopyOptions) any {
	if !v.IsValid() {
		return nil
	}
	for v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	// 1) Unwrap pgtype.* (punyamu, tetap)
	if out, ok := tryUnwrapPgType(v, path, opts); ok {
		return out
	}

	// 2) time.Time → hormati stringify rules
	if v.Kind() == reflect.Struct && v.Type() == timeType {
		return stringifyIfNeeded(path, v, opts)
	}

	switch v.Kind() {
	case reflect.Struct:
		// **Baru**: kalau struct ini implement json.Marshaler atau encoding.TextMarshaler,
		// kita pakai representasi JSON-nya langsung (agar “tercopy” utuh).
		if v.CanInterface() {
			if _, ok := v.Interface().(json.Marshaler); ok {
				return marshalToIface(v.Interface())
			}
			if tm, ok := v.Interface().(interface{ MarshalText() ([]byte, error) }); ok {
				if b, err := tm.MarshalText(); err == nil {
					return string(b)
				}
			}
		}
		// Fallback: pecah field (seperti sebelumnya)
		out := make(map[string]any)
		t := v.Type()
		for i := 0; i < t.NumField(); i++ {
			sf := t.Field(i)
			if sf.PkgPath != "" { // unexported
				continue
			}
			name, skip := jsonFieldName(sf, opts.UseJSONTags)
			if skip {
				continue
			}
			child := joinPath(path, name)
			out[name] = transformValue(v.Field(i), child, opts)
		}
		return out

	case reflect.Slice, reflect.Array:
		// []byte → encode seperti BYTEA agar konsisten & aman JSON
		if v.Kind() == reflect.Slice && v.Type().Elem().Kind() == reflect.Uint8 {
			b := make([]byte, v.Len())
			reflect.Copy(reflect.ValueOf(b), v)
			if strings.ToLower(opts.ByteaEncoding) == "hex" {
				return "0x" + hex.EncodeToString(b)
			}
			return base64.StdEncoding.EncodeToString(b)
		}
		n := v.Len()
		out := make([]any, n)
		for i := 0; i < n; i++ {
			child := fmt.Sprintf("%s[%d]", path, i)
			out[i] = transformValue(v.Index(i), child, opts)
		}
		return out

	case reflect.Map:
		// Map key non-string tidak valid di JSON → roundtrip supaya aman
		// (encoder akan konversi melalui MarshalJSON/Marshaler bila ada; kalau tidak, bisa gagal)
		if v.Type().Key().Kind() != reflect.String {
			if v.CanInterface() {
				return marshalToIface(v.Interface())
			}
			return marshalToIface(toInterface(v))
		}
		out := make(map[string]any)
		iter := v.MapRange()
		for iter.Next() {
			k := iter.Key().String()
			child := joinPath(path, k)
			out[k] = transformValue(iter.Value(), child, opts)
		}
		return out

	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.String:
		if _, ok := findRuleForPath(path, opts.Stringify); ok {
			// Kalau ada rule di path ini, paksa jadi string
			return fmt.Sprint(v.Interface())
		}
		return v.Interface()

	default:
		// **Baru**: untuk tipe lain (chan, func, complex, dsb.) atau custom types yang
		// tidak kita handle, pakai roundtrip JSON biar “tercopy” kalau memungkinkan.
		if v.CanInterface() {
			return marshalToIface(v.Interface())
		}
		return marshalToIface(toInterface(v))
	}
}

// Jika path match rule → ubah time ke string; selain itu kembalikan time.Time biasa (biar JSON encode sebagai RFC3339)
func stringifyIfNeeded(path string, v reflect.Value, opts *ObjectCopyOptions) any {
	var t time.Time
	if v.Type() == timeType {
		t = v.Interface().(time.Time)
	} else if v.Type() == timePtrType {
		if v.IsNil() {
			return nil
		}
		t = v.Elem().Interface().(time.Time)
	} else {
		return v.Interface()
	}

	if rule, ok := findRuleForPath(path, opts.Stringify); ok {
		if rule.IfZeroUseEmpty && t.IsZero() {
			return ""
		}
		format := rule.TimeFormat
		if format == "" {
			format = opts.DefaultTimeFormat
		}
		return t.Format(format)
	}
	// kalau tidak ada rule, kembalikan object time.Time (biar JSON pakai RFC3339)
	return t
}

// === Helpers ===

func jsonFieldName(sf reflect.StructField, useJSONTag bool) (name string, skip bool) {
	if !useJSONTag {
		return sf.Name, false
	}
	tag := sf.Tag.Get("json")
	if tag == "-" {
		return "", true
	}
	if tag == "" {
		return sf.Name, false
	}
	parts := strings.Split(tag, ",")
	n := parts[0]
	if n == "" {
		return sf.Name, false
	}
	return n, false
}

func joinPath(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}

// Normalisasi path:
// - ganti [<angka>] → []
// - untuk map wildcard → caller mendefinisikan rule dengan ".*" pada segmen itu
func normalizeIndexes(p string) string {
	var sb strings.Builder
	runes := []rune(p)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '[' {
			// konsumsi sampai ']'
			for i < len(runes) && runes[i] != ']' {
				i++
			}
			// ganti dengan []
			sb.WriteString("[]")
			continue
		}
		sb.WriteRune(runes[i])
	}
	return sb.String()
}

func mapWildcard(p string) string {
	// Ubah setiap segmen konkret menjadi ".*" jika ingin wild-match map key.
	// Implementasi sederhana: caller menulis rule dengan ".*" manual;
	// di sisi matcher, kita juga cek varian yang mengganti segmen terakhir jadi ".*"
	return p
}

func findRuleForPath(curPath string, rules map[string]StringifyRule) (StringifyRule, bool) {
	// Exact
	if r, ok := rules[curPath]; ok {
		return r, true
	}
	// Normalisasi indeks → []
	norm := normalizeIndexes(curPath)
	if r, ok := rules[norm]; ok {
		return r, true
	}
	// Coba varian wildcard map: ganti segmen terakhir jadi ".*"
	if r, ok := rules[replaceLastSegmentWithStar(norm)]; ok {
		return r, true
	}
	// Coba semua segmen jadi star (agresif, tapi berguna)
	if r, ok := rules[allSegmentsStar(norm)]; ok {
		return r, true
	}
	return StringifyRule{}, false
}

func replaceLastSegmentWithStar(p string) string {
	segs := strings.Split(p, ".")
	if len(segs) == 0 {
		return p
	}
	segs[len(segs)-1] = "*"
	return strings.Join(segs, ".")
}

func allSegmentsStar(p string) string {
	segs := strings.Split(p, ".")
	for i := range segs {
		// Jangan ubah [] (indeks wildcard)
		if segs[i] != "[]" {
			segs[i] = "*"
		}
	}
	return strings.Join(segs, ".")
}

func toInterface(v reflect.Value) any {
	return v.Interface()
}

// tryUnwrapPgType meng-unbox nilai pgtype.* menjadi tipe Go biasa atau string
func tryUnwrapPgType(v reflect.Value, path string, opts *ObjectCopyOptions) (any, bool) {
	t := v.Type()
	if !strings.Contains(t.PkgPath(), "github.com/jackc/pgx/v5/pgtype") {
		return nil, false
	}
	name := t.Name()

	// helper get field safely
	getf := func(field string) (reflect.Value, bool) {
		f := v.FieldByName(field)
		if !f.IsValid() {
			return reflect.Value{}, false
		}
		return f, true
	}
	// helper check valid
	isValid := func() (bool, bool) {
		if f, ok := getf("Valid"); ok && f.Kind() == reflect.Bool {
			return f.Bool(), true
		}
		return true, false // jika tak ada Valid, anggap valid
	}
	// handle null policy
	nullReturn := func(kind string) any {
		switch kind {
		case "string":
			if opts.PgtypeNullStringAsEmpty {
				return ""
			}
			return nil
		case "time":
			if opts.PgtypeNullTimeAsEmpty {
				return ""
			}
			return nil
		case "bool":
			if opts.PgtypeNullBoolAsFalse {
				return false
			}
			return nil
		case "int":
			if opts.PgtypeNullIntAsZero {
				return int64(0)
			}
			return nil
		case "float":
			if opts.PgtypeNullFloatAsZero {
				return float64(0)
			}
			return nil
		case "numeric":
			if opts.PgtypeNullNumericAsZero {
				if opts.NumericAsString {
					return "0"
				}
				return float64(0)
			}
			return nil
		case "bytes":
			return nil
		default:
			return nil
		}
	}

	// --- String-like ---
	switch name {
	case "Text", "Varchar", "BPChar", "XIDText", "Name":
		if ok, _ := isValid(); !ok {
			return nullReturn("string"), true
		}
		if s, ok := getf("String"); ok && s.Kind() == reflect.String {
			return s.String(), true
		}
		return fmt.Sprint(v.Interface()), true
	}

	// --- Bool ---
	if name == "Bool" {
		if ok, _ := isValid(); !ok {
			return nullReturn("bool"), true
		}
		if b, ok := getf("Bool"); ok && b.Kind() == reflect.Bool {
			return b.Bool(), true
		}
		// fallback: some versions use "Bool" or "Value"
		if val, ok := getf("Value"); ok && val.IsValid() && val.Kind() == reflect.Bool {
			return val.Bool(), true
		}
		return false, true
	}

	// --- Integers ---
	switch name {
	case "Int2", "Int4", "Int8":
		if ok, _ := isValid(); !ok {
			return nullReturn("int"), true
		}
		// prioritaskan Int64, jatuh ke Int32/Int16
		if f, ok := getf("Int64"); ok && f.Kind() == reflect.Int64 {
			return f.Int(), true
		}
		if f, ok := getf("Int32"); ok && f.Kind() == reflect.Int32 {
			return int64(f.Int()), true
		}
		if f, ok := getf("Int16"); ok && f.Kind() == reflect.Int16 {
			return int64(f.Int()), true
		}
		return int64(0), true
	}

	// --- Floats ---
	switch name {
	case "Float4", "Float8":
		if ok, _ := isValid(); !ok {
			return nullReturn("float"), true
		}
		if f, ok := getf("Float64"); ok && (f.Kind() == reflect.Float64 || f.Kind() == reflect.Float32) {
			return f.Float(), true
		}
		if f, ok := getf("Float32"); ok && (f.Kind() == reflect.Float32 || f.Kind() == reflect.Float64) {
			return f.Float(), true
		}
		return float64(0), true
	}

	// --- Numeric (besar/decimal) ---
	if name == "Numeric" {
		if ok, _ := isValid(); !ok {
			return nullReturn("numeric"), true
		}
		// Cara paling aman → gunakan String() bawaan
		s := fmt.Sprint(v.Interface())
		if opts.NumericAsString {
			return s, true
		}
		// attempt parse ke float64 (bisa kehilangan presisi)
		if fv, err := strconv.ParseFloat(s, 64); err == nil {
			return fv, true
		}
		// fallback: tetap string
		return s, true
	}

	// --- Time-like (Timestamp/Date/Time with/without tz) ---
	switch name {
	case "Timestamptz", "Timestamp", "Timetz", "Time", "Date":
		if ok, _ := isValid(); !ok {
			return nullReturn("time"), true
		}

		// Banyak pgtype waktu punya field 'Time time.Time'
		if tf, ok := getf("Time"); ok && tf.IsValid() && tf.Type() == reflect.TypeOf(time.Time{}) {
			// Hormati rule stringify (format)
			return stringifyIfNeeded(path, tf, opts), true
		}

		// pgtype.Date variasi: Year, Month, Day
		yF, yOk := getf("Year")
		mF, mOk := getf("Month")
		dF, dOk := getf("Day")
		if yOk && mOk && dOk && (yF.Kind() == reflect.Int32 || yF.Kind() == reflect.Int || yF.Kind() == reflect.Int64) {
			t := time.Date(int(yF.Int()), time.Month(mF.Int()), int(dF.Int()), 0, 0, 0, 0, time.UTC)
			rv := reflect.ValueOf(t)
			return stringifyIfNeeded(path, rv, opts), true
		}

		// Timetz/Time variasi: Hour, Minute, Second, Microseconds
		if hF, ok := getf("Hour"); ok {
			minF, _ := getf("Minute")
			secF, _ := getf("Second")
			usF, _ := getf("Microseconds")
			t := time.Date(1970, 1, 1, int(hF.Int()), int(minF.Int()), int(secF.Int()), int(usF.Int())*1000, time.UTC)
			rv := reflect.ValueOf(t)
			return stringifyIfNeeded(path, rv, opts), true
		}

		// fallback: gunakan fmt
		return fmt.Sprint(v.Interface()), true
	}

	// --- UUID ---
	if name == "UUID" {
		if ok, _ := isValid(); !ok {
			return nil, true
		}
		if !opts.UUIDAsString {
			return v.Interface(), true
		}
		// pgtype.UUID biasanya punya field Bytes [16]byte
		if bf, ok := getf("Bytes"); ok && bf.IsValid() && bf.CanAddr() {
			// format ke string xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
			b := bf.Bytes()
			if len(b) == 0 && bf.Kind() == reflect.Array && bf.Len() == 16 {
				// array [16]byte
				b = make([]byte, 16)
				reflect.Copy(reflect.ValueOf(b), bf)
			}
			if len(b) == 16 {
				return formatUUID(b), true
			}
		}
		return fmt.Sprint(v.Interface()), true
	}

	// --- JSON / JSONB ---
	if name == "JSON" || name == "JSONB" {
		if ok, _ := isValid(); !ok {
			return nil, true
		}
		// Field Bytes []byte atau mungkin RawMessage
		if bf, ok := getf("Bytes"); ok && (bf.Kind() == reflect.Slice || bf.Kind() == reflect.Array) {
			raw := make([]byte, bf.Len())
			reflect.Copy(reflect.ValueOf(raw), bf)
			if opts.JSONAsRawMessage {
				return json.RawMessage(raw), true
			}
			var out any
			if len(raw) == 0 {
				return nil, true
			}
			if err := json.Unmarshal(raw, &out); err == nil {
				return out, true
			}
			// fallback: string
			return string(raw), true
		}
		return nil, true
	}

	// --- INET ---
	if name == "Inet" {
		if ok, _ := isValid(); !ok {
			return nil, true
		}
		if !opts.InetAsString {
			return v.Interface(), true
		}
		// Banyak implementasi punya field IPNet *net.IPNet
		if nf, ok := getf("IPNet"); ok && !nf.IsNil() {
			ipnet, _ := nf.Interface().(*net.IPNet)
			if ipnet != nil {
				return ipnet.String(), true
			}
		}
		// beberapa varian punya IP []byte, Mask int?
		if ipF, ok := getf("IP"); ok && ipF.IsValid() && ipF.Kind() == reflect.Slice {
			ip := make([]byte, ipF.Len())
			reflect.Copy(reflect.ValueOf(ip), ipF)
			return net.IP(ip).String(), true
		}
		return fmt.Sprint(v.Interface()), true
	}

	// --- MACADDR ---
	if name == "Macaddr" || name == "Macaddr8" {
		if ok, _ := isValid(); !ok {
			return nil, true
		}
		// Cari field Addr [6]byte atau [8]byte
		if af, ok := getf("Addr"); ok && af.IsValid() && af.Kind() == reflect.Array {
			n := af.Len()
			bytes := make([]byte, n)
			for i := 0; i < n; i++ {
				bytes[i] = byte(af.Index(i).Uint())
			}
			if n == 6 || n == 8 {
				return formatMAC(bytes), true
			}
		}
		return fmt.Sprint(v.Interface()), true
	}

	// --- BYTEA ---
	if name == "Bytea" {
		if ok, _ := isValid(); !ok {
			return nullReturn("bytes"), true
		}
		if bf, ok := getf("Bytes"); ok && bf.IsValid() && bf.Kind() == reflect.Slice {
			b := make([]byte, bf.Len())
			reflect.Copy(reflect.ValueOf(b), bf)
			if strings.ToLower(opts.ByteaEncoding) == "hex" {
				return "0x" + hex.EncodeToString(b), true
			}
			// default base64
			return base64.StdEncoding.EncodeToString(b), true
		}
		return nil, true
	}

	// --- OID (optional) ---
	if name == "OID" {
		if ok, _ := isValid(); !ok {
			return nil, true
		}
		if f, ok := getf("Uint32"); ok && (f.Kind() == reflect.Uint32 || f.Kind() == reflect.Uint64 || f.Kind() == reflect.Uint) {
			return uint32(f.Uint()), true
		}
		return fmt.Sprint(v.Interface()), true
	}

	// Tipe lain yang jarang → fallback printing
	return fmt.Sprint(v.Interface()), true
}

// ===== helpers =====

func formatUUID(b []byte) string {
	// b must be 16 bytes
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		binary.BigEndian.Uint32(b[0:4]),
		binary.BigEndian.Uint16(b[4:6]),
		binary.BigEndian.Uint16(b[6:8]),
		binary.BigEndian.Uint16(b[8:10]),
		b[10:16],
	)
}

func formatMAC(b []byte) string {
	var parts []string
	for _, x := range b {
		parts = append(parts, fmt.Sprintf("%02x", x))
	}
	return strings.Join(parts, ":")
}

// --- tambahkan helper ini ---
func marshalToIface(x any) any {
	// Gunakan JSON encoder bawaan untuk menghormati MarshalJSON/TextMarshaler,
	// sekaligus menormalkan struktur (map key -> string, dsb.)
	b, err := json.Marshal(x)
	if err != nil {
		// fallback keras: kalau benar-benar gagal, balikin nilai mentah
		return x
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return x
	}
	return out
}
