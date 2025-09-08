package copy

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"time"
)

// Copy menyalin/konversi dari src ke dst (rekursif, dynamic).
// dst harus pointer (ke struct / map / slice / tipe primitif).
func Copy(dst any, src any) error {
	if dst == nil {
		return fmt.Errorf("dst is nil")
	}
	dv := reflect.ValueOf(dst)
	if dv.Kind() != reflect.Ptr {
		return fmt.Errorf("dst must be a pointer, got %s", dv.Kind())
	}
	if !dv.Elem().CanSet() {
		return fmt.Errorf("dst is not settable")
	}
	return convertAssign(dv.Elem(), reflect.ValueOf(src))
}

// ====================== Core ======================

func convertAssign(dst, src reflect.Value) error {
	// Unwrap pointers (src)
	for src.Kind() == reflect.Ptr {
		if src.IsNil() {
			dst.Set(reflect.Zero(dst.Type()))
			return nil
		}
		src = src.Elem()
	}
	// Jika dst pointer, alokasikan dan kerjakan elemennya
	if dst.Kind() == reflect.Ptr {
		// Jangan alokasikan dulu; cek apakah src adalah pgtype.* null
		if src.Kind() == reflect.Struct {
			if isPg, isNull := isPgtypeNull(src); isPg && isNull {
				dst.Set(reflect.Zero(dst.Type())) // nil
				return nil
			}
		}
		// lanjut: baru alokasikan bila tidak null
		if dst.IsNil() {
			dst.Set(reflect.New(dst.Type().Elem()))
		}
		return convertAssign(dst.Elem(), src)
	}

	// Tipe langsung assignable
	if src.Type().AssignableTo(dst.Type()) {
		dst.Set(src)
		return nil
	}

	// 0) Prioritas: pgtype.AssignTo
	if handled, err := tryAssignViaPgtype(dst, src); handled {
		return err
	}

	// 1) Unwrap nullable untuk database/sql Null*
	if base, ok, valid := unwrapSQLNull(src); ok {
		if !valid {
			dst.Set(reflect.Zero(dst.Type()))
			return nil
		}
		src = base
		if src.Type().AssignableTo(dst.Type()) {
			dst.Set(src)
			return nil
		}
	}

	// 2) Jika target struct/map/slice dan sumber adalah string/[]byte yang "mirip JSON", unmarshal
	if (dst.Kind() == reflect.Struct || dst.Kind() == reflect.Map || dst.Kind() == reflect.Slice) && looksLikeJSON(src) {
		return convertJSON(dst, src)
	}

	// 3) Number ↔ Number
	if isNumber(dst.Kind()) && isNumber(src.Kind()) {
		return numberToNumber(dst, src)
	}

	// 4) String ↔ []byte
	if dst.Kind() == reflect.String && isBytes(src) {
		dst.SetString(string(src.Bytes()))
		return nil
	}
	if isBytes(dst) && src.Kind() == reflect.String {
		dst.SetBytes([]byte(src.String()))
		return nil
	}

	// 5) time.Time & string/int64 (unix)
	if dst.Type() == reflect.TypeOf(time.Time{}) {
		return toTime(dst, src)
	}
	if src.Type() == reflect.TypeOf(time.Time{}) {
		return fromTime(dst, src)
	}

	// 6) Slice/Array
	if dst.Kind() == reflect.Slice {
		return sliceConvert(dst, src)
	}
	if dst.Kind() == reflect.Array {
		return arrayConvert(dst, src)
	}

	// 7) Map[string]T
	if dst.Kind() == reflect.Map {
		return mapConvert(dst, src)
	}

	// 8) Struct
	if dst.Kind() == reflect.Struct {
		return structConvert(dst, src)
	}

	// 9) Fallback (string/bool conversion, dll.)
	return fallbackConvert(dst, src)
}

// ====================== pgtype AssignTo (v5.5.5) ======================

// tryAssignViaPgtype memanggil AssignTo(&dst) bila ada.
// Jika AssignTo error (contoh: Numeric → float64 overflow), coba AssignTo ke string,
// lalu lanjutkan konversi biasa dari string ke tipe dst bila memungkinkan.
func tryAssignViaPgtype(dst, src reflect.Value) (handled bool, err error) {
	// Cari method AssignTo pada value atau addressable value
	m := src.MethodByName("AssignTo")
	if !m.IsValid() && src.CanAddr() {
		m = src.Addr().MethodByName("AssignTo")
	}
	if !m.IsValid() && src.Kind() == reflect.Ptr && !src.IsNil() {
		m = src.Elem().MethodByName("AssignTo")
		if !m.IsValid() && src.Elem().CanAddr() {
			m = src.Elem().Addr().MethodByName("AssignTo")
		}
	}
	if !m.IsValid() {
		return false, nil
	}

	// 1) Coba langsung ke tipe dst
	dstPtr := reflect.New(dst.Type())
	out := m.Call([]reflect.Value{dstPtr})
	if len(out) == 1 && !out[0].IsNil() {
		// gagal → coba AssignTo ke string dulu (universal)
		var s string
		sPtr := reflect.New(reflect.TypeOf(s))
		out2 := m.Call([]reflect.Value{sPtr})
		if len(out2) == 1 && !out2[0].IsNil() {
			// tetap gagal: biarkan aturan lain menangani
			return false, nil
		}
		// sukses ke string → konversi string ke tipe dst (JSON/number/time/string)
		tmp := sPtr.Elem()
		if dst.Kind() == reflect.String {
			dst.Set(tmp)
			return true, nil
		}
		if (dst.Kind() == reflect.Struct || dst.Kind() == reflect.Map || dst.Kind() == reflect.Slice) && looksLikeJSON(tmp) {
			return true, convertJSON(dst, tmp)
		}
		if isNumber(dst.Kind()) {
			return true, numberToNumber(dst, tmpToNumber(tmp))
		}
		if dst.Type() == reflect.TypeOf(time.Time{}) {
			return true, toTime(dst, tmp)
		}
		return true, fallbackConvert(dst, tmp)
	}

	// 2) AssignTo langsung sukses
	dst.Set(dstPtr.Elem())
	return true, nil
}

func tmpToNumber(s reflect.Value) reflect.Value {
	if s.Kind() == reflect.String {
		if f, err := strconv.ParseFloat(s.String(), 64); err == nil {
			return reflect.ValueOf(f)
		}
	}
	return s
}

// ====================== Nullable unwrap (database/sql) ======================

func unwrapSQLNull(src reflect.Value) (base reflect.Value, ok bool, valid bool) {
	switch v := src.Interface().(type) {
	case sql.NullString:
		return reflect.ValueOf(v.String), true, v.Valid
	case sql.NullInt32:
		return reflect.ValueOf(int32(v.Int32)), true, v.Valid
	case sql.NullInt64:
		return reflect.ValueOf(v.Int64), true, v.Valid
	case sql.NullFloat64:
		return reflect.ValueOf(v.Float64), true, v.Valid
	case sql.NullBool:
		return reflect.ValueOf(v.Bool), true, v.Valid
	case sql.NullTime:
		return reflect.ValueOf(v.Time), true, v.Valid
	default:
		return reflect.Value{}, false, false
	}
}

// ====================== JSON helpers (tanpa pgtype JSON/JSONB) ======================

func looksLikeJSON(v reflect.Value) bool {
	if isBytes(v) {
		b := bytes.TrimSpace(v.Bytes())
		return len(b) > 0 && (b[0] == '{' || b[0] == '[')
	}
	if v.Kind() == reflect.String {
		s := bytes.TrimSpace([]byte(v.String()))
		return len(s) > 0 && (s[0] == '{' || s[0] == '[')
	}
	return false
}

func convertJSON(dst, src reflect.Value) error {
	var raw []byte
	if isBytes(src) {
		raw = src.Bytes()
	} else if src.Kind() == reflect.String {
		raw = []byte(src.String())
	} else {
		// sebagai safeguard untuk kasus AssignTo mengembalikan tipe lain
		raw = []byte(fmt.Sprint(src.Interface()))
	}

	switch {
	case dst.Kind() == reflect.String:
		dst.SetString(string(raw))
		return nil
	case isBytes(dst):
		dst.SetBytes(raw)
		return nil
	default:
		tmp := reflect.New(dst.Type()).Interface()
		if err := json.Unmarshal(raw, tmp); err != nil {
			return fmt.Errorf("json unmarshal to %s: %w", dst.Type(), err)
		}
		dst.Set(reflect.ValueOf(tmp).Elem())
		return nil
	}
}

// ====================== Numbers ======================

func isNumber(k reflect.Kind) bool {
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

func numberToNumber(dst, src reflect.Value) error {
	var f float64
	switch src.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		f = float64(src.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		f = float64(src.Uint())
	case reflect.Float32, reflect.Float64:
		f = src.Convert(reflect.TypeOf(float64(0))).Float()
	case reflect.String:
		parsed, err := strconv.ParseFloat(src.String(), 64)
		if err != nil {
			return err
		}
		f = parsed
	default:
		return fmt.Errorf("unsupported number kind: %s", src.Kind())
	}

	switch dst.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		dst.SetInt(int64(f))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if f < 0 {
			f = 0
		}
		dst.SetUint(uint64(f))
	case reflect.Float32, reflect.Float64:
		dst.SetFloat(f)
	default:
		return fmt.Errorf("unsupported destination number kind: %s", dst.Kind())
	}
	return nil
}

// ====================== time.Time ======================

func toTime(dst, src reflect.Value) error {
	switch src.Kind() {
	case reflect.String:
		if t, err := time.Parse(time.RFC3339, src.String()); err == nil {
			dst.Set(reflect.ValueOf(t))
			return nil
		}
		if sec, err := strconv.ParseInt(src.String(), 10, 64); err == nil {
			dst.Set(reflect.ValueOf(time.Unix(sec, 0)))
			return nil
		}
	case reflect.Int, reflect.Int64:
		dst.Set(reflect.ValueOf(time.Unix(src.Int(), 0)))
		return nil
	case reflect.Struct:
		if src.Type() == reflect.TypeOf(time.Time{}) {
			dst.Set(src)
			return nil
		}
	}
	return fmt.Errorf("cannot convert %s to time.Time", src.Kind())
}

func fromTime(dst, src reflect.Value) error {
	t := src.Interface().(time.Time)
	switch dst.Kind() {
	case reflect.String:
		dst.SetString(t.Format(time.RFC3339))
		return nil
	case reflect.Int, reflect.Int64:
		dst.SetInt(t.Unix())
		return nil
	}
	return fmt.Errorf("cannot convert time.Time to %s", dst.Kind())
}

// ====================== bytes ======================

func isBytes(v reflect.Value) bool {
	return v.Kind() == reflect.Slice && v.Type().Elem().Kind() == reflect.Uint8
}

// ====================== slice/array/map/struct ======================

func sliceConvert(dst, src reflect.Value) error {
	switch src.Kind() {
	case reflect.Slice, reflect.Array:
		out := reflect.MakeSlice(dst.Type(), 0, src.Len())
		for i := 0; i < src.Len(); i++ {
			elem := reflect.New(dst.Type().Elem()).Elem()
			if err := convertAssign(elem, src.Index(i)); err != nil {
				return fmt.Errorf("slice index %d: %w", i, err)
			}
			out = reflect.Append(out, elem)
		}
		dst.Set(out)
		return nil
	default:
		// single → slice len 1
		elem := reflect.New(dst.Type().Elem()).Elem()
		if err := convertAssign(elem, src); err != nil {
			return err
		}
		out := reflect.MakeSlice(dst.Type(), 1, 1)
		out.Index(0).Set(elem)
		dst.Set(out)
		return nil
	}
}

func arrayConvert(dst, src reflect.Value) error {
	if src.Kind() != reflect.Array && src.Kind() != reflect.Slice {
		return fmt.Errorf("cannot convert %s to array", src.Kind())
	}
	n := dst.Len()
	for i := 0; i < n && i < src.Len(); i++ {
		if err := convertAssign(dst.Index(i), src.Index(i)); err != nil {
			return fmt.Errorf("array index %d: %w", i, err)
		}
	}
	return nil
}

func mapConvert(dst, src reflect.Value) error {
	if dst.Type().Key().Kind() != reflect.String {
		return fmt.Errorf("only map[string]T supported")
	}
	out := reflect.MakeMapWithSize(dst.Type(), 0)

	switch src.Kind() {
	case reflect.Map:
		iter := src.MapRange()
		for iter.Next() {
			k := iter.Key()
			var keyStr string
			if k.Kind() == reflect.String {
				keyStr = k.String()
			} else {
				keyStr = fmt.Sprint(k.Interface())
			}
			val := reflect.New(dst.Type().Elem()).Elem()
			if err := convertAssign(val, iter.Value()); err != nil {
				return fmt.Errorf("map key %q: %w", keyStr, err)
			}
			out.SetMapIndex(reflect.ValueOf(keyStr), val)
		}
	case reflect.Struct:
		tmp := structToStringMap(src) // pakai prioritas db > json
		iter := tmp.MapRange()
		for iter.Next() {
			val := reflect.New(dst.Type().Elem()).Elem()
			if err := convertAssign(val, iter.Value()); err != nil {
				return err
			}
			out.SetMapIndex(iter.Key(), val)
		}
	default:
		return fmt.Errorf("cannot convert %s to map", src.Kind())
	}

	dst.Set(out)
	return nil
}

// Struct ← Struct / Map
func structConvert(dst, src reflect.Value) error {
	switch src.Kind() {
	case reflect.Struct:
		srcFields := indexStructFields(src.Type()) // db > json > Name
		for i := 0; i < dst.NumField(); i++ {
			df := dst.Field(i)
			if !df.CanSet() {
				continue
			}
			ft := dst.Type().Field(i)
			nameCandidates := fieldNames(ft)
			var sv reflect.Value
			found := false
			for _, nm := range nameCandidates {
				if idx, ok := srcFields[nm]; ok {
					sv = src.Field(idx)
					found = true
					break
				}
			}
			if !found {
				continue
			}
			if err := convertAssign(df, sv); err != nil {
				return fmt.Errorf("field %s: %w", ft.Name, err)
			}
		}
		return nil

	case reflect.Map:
		if src.Type().Key().Kind() != reflect.String {
			return fmt.Errorf("map key must be string to fill struct")
		}
		keys := src.MapKeys()
		for i := 0; i < dst.NumField(); i++ {
			df := dst.Field(i)
			if !df.CanSet() {
				continue
			}
			ft := dst.Type().Field(i)
			nameCandidates := fieldNames(ft)
			var mv reflect.Value
			found := false
			for _, k := range keys {
				ks := k.String()
				for _, cand := range nameCandidates {
					if equalFold(ks, cand) {
						mv = src.MapIndex(k)
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if !found {
				continue
			}
			if err := convertAssign(df, mv); err != nil {
				return fmt.Errorf("field %s: %w", ft.Name, err)
			}
		}
		return nil
	default:
		return fmt.Errorf("cannot convert %s to struct", src.Kind())
	}
}

// ====================== Field indexing & names (db > json) ======================

func indexStructFields(t reflect.Type) map[string]int {
	m := make(map[string]int, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		for _, nm := range fieldNames(f) {
			if _, exists := m[nm]; !exists {
				m[nm] = i
			}
		}
	}
	return m
}

// PRIORITAS: db:"" → json:"" → NamaField → lowerFirst(NamaField)
func fieldNames(f reflect.StructField) []string {
	var out []string
	if d := f.Tag.Get("db"); d != "" && d != "-" {
		out = append(out, tagHead(d))
	}
	if j := f.Tag.Get("json"); j != "" && j != "-" {
		out = append(out, tagHead(j))
	}
	out = append(out, f.Name)
	out = append(out, lowerFirst(f.Name))
	return out
}

func tagHead(tag string) string {
	for i := 0; i < len(tag); i++ {
		if tag[i] == ',' {
			return tag[:i]
		}
	}
	return tag
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	if 'A' <= r[0] && r[0] <= 'Z' {
		r[0] = r[0] - 'A' + 'a'
	}
	return string(r)
}

func equalFold(a, b string) bool {
	return bytes.EqualFold([]byte(a), []byte(b))
}

func structToStringMap(v reflect.Value) reflect.Value {
	m := reflect.MakeMapWithSize(reflect.TypeOf(map[string]any{}), v.NumField())
	idx := indexStructFields(v.Type())
	for name, i := range idx {
		m.SetMapIndex(reflect.ValueOf(name), v.Field(i))
	}
	return m
}

// ====================== Fallback ======================

func fallbackConvert(dst, src reflect.Value) error {
	switch dst.Kind() {
	case reflect.String:
		dst.SetString(fmt.Sprint(src.Interface()))
		return nil
	case reflect.Bool:
		switch src.Kind() {
		case reflect.String:
			b, err := strconv.ParseBool(src.String())
			if err != nil {
				return err
			}
			dst.SetBool(b)
			return nil
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			dst.SetBool(src.Int() != 0)
			return nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			dst.SetBool(src.Uint() != 0)
			return nil
		case reflect.Float32, reflect.Float64:
			dst.SetBool(src.Float() != 0)
			return nil
		}
	}
	return fmt.Errorf("no conversion rule from %s to %s", src.Type(), dst.Type())
}

// isPgtypeNull: kalau v struct punya field Valid bool dan false → null.
func isPgtypeNull(v reflect.Value) (isPgtype bool, isNull bool) {
	//t := v.Type()
	if v.Kind() == reflect.Struct {
		if f := v.FieldByName("Valid"); f.IsValid() && f.Kind() == reflect.Bool {
			return true, !f.Bool()
		}
		// beberapa pgtype (array dsb.) mungkin bungkus pointer; coba elem addr juga
	}
	return false, false
}
