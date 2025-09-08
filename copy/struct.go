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

// ------------------------ Options & Rules ------------------------

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
	JSONAsRawMessage bool   // true=json.RawMessage; false=decode ke interface{}
	UUIDAsString     bool   // true="xxxxxxxx-xxxx-...."
	InetAsString     bool   // true="1.2.3.4/24"
	ByteaEncoding    string // "base64" (default) atau "hex"

	// (Opsional) alias path → ganti nama key saat memecah struct src
	KeyAliases map[string]string // map["customer.phone"] = "phone_number"
}

var timeType = reflect.TypeOf(time.Time{})
var timePtrType = reflect.TypeOf(&time.Time{})

// ------------------------ Public API ------------------------

// Struct menyalin dari src ke dst dengan aturan/opsi.
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

// ------------------------ Core Transformer ------------------------

func transformValue(v reflect.Value, path string, opts *ObjectCopyOptions) any {
	if !v.IsValid() {
		return nil
	}

	// Unwrap interface/pointer secara bertahap
	for v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		// Jika pointer ke pgtype.*, langsung unbox
		if v.Kind() == reflect.Pointer && strings.Contains(v.Type().Elem().PkgPath(), "github.com/jackc/pgx/v5/pgtype") {
			if out, ok := tryUnwrapPgType(v.Elem(), path, opts); ok {
				return out
			}
		}
		v = v.Elem()
	}

	// Unwrap pgtype.* (non-pointer)
	if out, ok := tryUnwrapPgType(v, path, opts); ok {
		return out
	}

	// time.Time → hormati stringify rules
	if v.Kind() == reflect.Struct && v.Type() == timeType {
		return stringifyIfNeeded(path, v, opts)
	}

	switch v.Kind() {
	case reflect.Struct:
		// Hormati custom marshaller jika ada
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
		// Fallback: pecah field ke map[string]any
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
			alias := name
			if opts.KeyAliases != nil {
				if a, ok := opts.KeyAliases[joinPath(path, name)]; ok && a != "" {
					alias = a
				}
			}
			child := joinPath(path, alias)
			out[alias] = transformValue(v.Field(i), child, opts)
		}
		return out

	case reflect.Slice, reflect.Array:
		// []byte → encode aman JSON
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
		// Map key non-string → roundtrip agar valid JSON
		if v.Type().Key().Kind() != reflect.String {
			if v.CanInterface() {
				return marshalToIface(v.Interface())
			}
			return marshalToIface(v.Interface())
		}
		out := make(map[string]any)
		iter := v.MapRange()
		for iter.Next() {
			k := iter.Key().String()
			alias := k
			if opts.KeyAliases != nil {
				if a, ok := opts.KeyAliases[joinPath(path, k)]; ok && a != "" {
					alias = a
				}
			}
			child := joinPath(path, alias)
			out[alias] = transformValue(iter.Value(), child, opts)
		}
		return out

	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.String:
		// Paksa stringify bila ada rule di path ini
		if _, ok := findRuleForPath(path, opts.Stringify); ok {
			return fmt.Sprint(v.Interface())
		}
		return v.Interface()

	default:
		// Tipe lain → roundtrip JSON supaya tetap “tercopy” bila memungkinkan
		if v.CanInterface() {
			return marshalToIface(v.Interface())
		}
		return marshalToIface(v.Interface())
	}
}

// Jika path match rule → ubah time ke string; selain itu kembalikan time.Time
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
	return t
}

// ------------------------ pgtype Unwrapper ------------------------

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
		// fallback: some versions use "Value"
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
		if f, ok := getf("Int64"); ok && f.Kind() == reflect.Int64 {
			return f.Int(), true
		}
		if f, ok := getf("Int32"); ok {
			return int64(f.Int()), true
		}
		if f, ok := getf("Int16"); ok {
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
		if f, ok := getf("Float64"); ok {
			return f.Float(), true
		}
		if f, ok := getf("Float32"); ok {
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
		if fv, err := strconv.ParseFloat(s, 64); err == nil {
			return fv, true
		}
		return s, true
	}

	// --- Time-like (Timestamp/Date/Time with/without tz) ---
	switch name {
	case "Timestamptz", "Timestamp", "Timetz", "Time", "Date":
		if ok, _ := isValid(); !ok {
			return nullReturn("time"), true
		}

		// Banyak pgtype waktu punya field 'Time time.Time'
		if tf, ok := getf("Time"); ok && tf.IsValid() && tf.Type() == timeType {
			return stringifyIfNeeded(path, tf, opts), true
		}

		// pgtype.Date variasi: Year, Month, Day
		yF, yOk := getf("Year")
		mF, mOk := getf("Month")
		dF, dOk := getf("Day")
		if yOk && mOk && dOk {
			t := time.Date(int(yF.Int()), time.Month(mF.Int()), int(dF.Int()), 0, 0, 0, 0, time.UTC)
			return stringifyIfNeeded(path, reflect.ValueOf(t), opts), true
		}

		// Timetz/Time variasi: Hour, Minute, Second, Microseconds
		if hF, ok := getf("Hour"); ok {
			minF, _ := getf("Minute")
			secF, _ := getf("Second")
			usF, _ := getf("Microseconds")
			t := time.Date(1970, 1, 1, int(hF.Int()), int(minF.Int()), int(secF.Int()), int(usF.Int())*1000, time.UTC)
			return stringifyIfNeeded(path, reflect.ValueOf(t), opts), true
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
		if bf, ok := getf("Bytes"); ok && bf.IsValid() && bf.CanAddr() {
			b := bf.Bytes()
			if len(b) == 0 && bf.Kind() == reflect.Array && bf.Len() == 16 {
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
		if nf, ok := getf("IPNet"); ok && !nf.IsNil() {
			ipnet, _ := nf.Interface().(*net.IPNet)
			if ipnet != nil {
				return ipnet.String(), true
			}
		}
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
			return base64.StdEncoding.EncodeToString(b), true
		}
		return nil, true
	}

	// --- OID (opsional) ---
	if name == "OID" {
		if ok, _ := isValid(); !ok {
			return nil, true
		}
		if f, ok := getf("Uint32"); ok && (f.Kind() == reflect.Uint32 || f.Kind() == reflect.Uint64 || f.Kind() == reflect.Uint) {
			return uint32(f.Uint()), true
		}
		return fmt.Sprint(v.Interface()), true
	}

	// Tipe pgtype lain → fallback string
	return fmt.Sprint(v.Interface()), true
}

// ------------------------ Helpers ------------------------

func jsonFieldName(sf reflect.StructField, useJSONTag bool) (name string, skip bool) {
	if !useJSONTag {
		return sf.Name, false
	}
	tag := sf.Tag.Get("json")
	if tag == "-" {
		return "", true
	}
	if tag != "" {
		parts := strings.Split(tag, ",")
		n := parts[0]
		if n != "" {
			return n, false
		}
	}
	// Fallback: snake_case dari nama field Go
	return toSnakeCase(sf.Name), false
}

func joinPath(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}

// Normalisasi path: ganti [<angka>] → []
func normalizeIndexes(p string) string {
	var sb strings.Builder
	runes := []rune(p)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '[' {
			for i < len(runes) && runes[i] != ']' {
				i++
			}
			sb.WriteString("[]")
			continue
		}
		sb.WriteRune(runes[i])
	}
	return sb.String()
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
		if segs[i] != "[]" {
			segs[i] = "*"
		}
	}
	return strings.Join(segs, ".")
}

func findRuleForPath(curPath string, rules map[string]StringifyRule) (StringifyRule, bool) {
	if rules == nil {
		return StringifyRule{}, false
	}
	if r, ok := rules[curPath]; ok {
		return r, true
	}
	norm := normalizeIndexes(curPath)
	if r, ok := rules[norm]; ok {
		return r, true
	}
	if r, ok := rules[replaceLastSegmentWithStar(norm)]; ok {
		return r, true
	}
	if r, ok := rules[allSegmentsStar(norm)]; ok {
		return r, true
	}
	return StringifyRule{}, false
}

func toSnakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			prev := rune(s[i-1])
			if (prev >= 'a' && prev <= 'z') || (prev >= '0' && prev <= '9') {
				b.WriteByte('_')
			}
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}

func marshalToIface(x any) any {
	b, err := json.Marshal(x)
	if err != nil {
		return x
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return x
	}
	return out
}

func formatUUID(b []byte) string {
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
