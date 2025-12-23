package copy

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
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
	// Unwrap pointer src
	for src.Kind() == reflect.Ptr {
		if src.IsNil() {
			// src nil → dst zero
			dst.Set(reflect.Zero(dst.Type()))
			return nil
		}
		src = src.Elem()
	}

	// Jika dst pointer: handle NULL pgtype dulu agar bisa set nil
	if dst.Kind() == reflect.Ptr {
		if isPg, isNull := isPgtypeNullValue(src); isPg && isNull {
			// src adalah pgtype NULL → dst pointer = nil
			dst.Set(reflect.Zero(dst.Type()))
			return nil
		}
		// alok lalu teruskan ke elem
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

	// 1) pgtype.AssignTo (prioritas utama) - pointer-aware & fallback ke string
	if handled, err := tryAssignViaPgtype(dst, src); handled {
		return err
	}

	// 2) Unwrap database/sql Null*
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

	// 3) Jika target struct/map/slice dan sumber adalah string/[]byte "mirip JSON" → unmarshal
	if (dst.Kind() == reflect.Struct || dst.Kind() == reflect.Map || dst.Kind() == reflect.Slice) && looksLikeJSON(src) {
		return convertJSON(dst, src)
	}

	// 4) Number ↔ Number (termasuk src string numeric)
	if isNumber(dst.Kind()) && (isNumber(src.Kind()) || src.Kind() == reflect.String) {
		return numberToNumber(dst, src)
	}

	// 5) String ↔ []byte
	if dst.Kind() == reflect.String && isBytes(src) {
		dst.SetString(string(src.Bytes()))
		return nil
	}
	if isBytes(dst) && src.Kind() == reflect.String {
		dst.SetBytes([]byte(src.String()))
		return nil
	}

	// 6) time.Time & string/int64 (unix) & pgtype timestamp (via toTime)
	if dst.Type() == reflect.TypeOf(time.Time{}) {
		return toTime(dst, src)
	}
	if src.Type() == reflect.TypeOf(time.Time{}) {
		return fromTime(dst, src)
	}

	// 7) Slice/Array
	if dst.Kind() == reflect.Slice {
		return sliceConvert(dst, src)
	}
	if dst.Kind() == reflect.Array {
		return arrayConvert(dst, src)
	}

	// 8) Map[string]T
	if dst.Kind() == reflect.Map {
		return mapConvert(dst, src)
	}

	// 9) Struct
	if dst.Kind() == reflect.Struct {
		return structConvert(dst, src)
	}

	// 10) Fallback generik
	return fallbackConvert(dst, src)
}

// ====================== pgtype AssignTo ======================

// isPgtypeNullValue: deteksi pola pgtype.* dengan field Valid=false (NULL).
func isPgtypeNullValue(src reflect.Value) (isPgtype bool, isNull bool) {
	v := src
	if v.Kind() == reflect.Ptr && !v.IsNil() {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return false, false
	}
	if f := v.FieldByName("Valid"); f.IsValid() && f.Kind() == reflect.Bool {
		return true, !f.Bool()
	}
	return false, false
}

// tryAssignViaPgtype memanggil AssignTo(&dst) bila ada.
// Jika gagal, coba AssignTo ke *string, lalu konversi ke tipe dst (JSON/number/time/fallback).
func tryAssignViaPgtype(dst, src reflect.Value) (handled bool, err error) {
	// Temukan method AssignTo di value / &value / elem / &elem
	m := methodAssignTo(src)
	if !m.IsValid() {
		return false, nil
	}

	// Siapkan argumen:
	// - jika dst pointer → langsung pakai dst (mis. *time.Time, *int64, *string, *[]byte)
	// - jika dst non-pointer → buat *T sementara
	var dstArg reflect.Value
	if dst.Kind() == reflect.Ptr {
		if dst.IsNil() {
			dst.Set(reflect.New(dst.Type().Elem()))
		}
		dstArg = dst
	} else {
		dstArg = reflect.New(dst.Type())
	}

	// Panggil AssignTo
	out := m.Call([]reflect.Value{dstArg})
	if len(out) == 1 && !out[0].IsNil() {
		// error: coba AssignTo ke *string
		var s string
		sPtr := reflect.New(reflect.TypeOf(s))
		out2 := m.Call([]reflect.Value{sPtr})
		if len(out2) == 1 && !out2[0].IsNil() {
			// tetap error → biar aturan lain yang coba
			return false, nil
		}
		tmp := sPtr.Elem() // string

		if dst.Kind() == reflect.String {
			dst.Set(tmp)
			return true, nil
		}
		// JSON?
		if (dst.Kind() == reflect.Struct || dst.Kind() == reflect.Map || dst.Kind() == reflect.Slice) && looksLikeJSON(tmp) {
			return true, convertJSON(dst, tmp)
		}
		// Number?
		if isNumber(dst.Kind()) {
			return true, numberToNumber(dst, tmp)
		}
		// time?
		if dst.Type() == reflect.TypeOf(time.Time{}) {
			return true, toTime(dst, tmp)
		}
		// []byte?
		if isBytes(dst) {
			dst.SetBytes([]byte(tmp.String()))
			return true, nil
		}
		// Fallback
		return true, fallbackConvert(dst, tmp)
	}

	// AssignTo sukses
	if dst.Kind() != reflect.Ptr {
		dst.Set(dstArg.Elem())
	}
	return true, nil
}

func methodAssignTo(v reflect.Value) reflect.Value {
	if m := v.MethodByName("AssignTo"); m.IsValid() {
		return m
	}
	if v.CanAddr() {
		if m := v.Addr().MethodByName("AssignTo"); m.IsValid() {
			return m
		}
	}
	if v.Kind() == reflect.Ptr && !v.IsNil() {
		e := v.Elem()
		if m := e.MethodByName("AssignTo"); m.IsValid() {
			return m
		}
		if e.CanAddr() {
			if m := e.Addr().MethodByName("AssignTo"); m.IsValid() {
				return m
			}
		}
	}
	return reflect.Value{}
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
		parsed, err := strconv.ParseFloat(strings.TrimSpace(src.String()), 64)
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
	// Fast path: sudah time.Time
	if src.Type() == reflect.TypeOf(time.Time{}) {
		dst.Set(src)
		return nil
	}

	// 1) Coba AssignTo(&time.Time) jika ada (pgtype.Timestamp/Timestamptz/Date)
	if m := methodAssignTo(src); m.IsValid() {
		var t time.Time
		out := m.Call([]reflect.Value{reflect.ValueOf(&t)})
		if len(out) == 1 && out[0].IsNil() {
			dst.Set(reflect.ValueOf(t))
			return nil
		}
		// kalau error karena NULL → zero time
		if len(out) == 1 && !out[0].IsNil() {
			if err, ok := out[0].Interface().(error); ok && strings.Contains(err.Error(), "NULL") {
				dst.Set(reflect.Zero(dst.Type()))
				return nil
			}
			// lanjut fallback lain
		}
	}

	// 2) String (RFC3339 / unix detik)
	switch src.Kind() {
	case reflect.String:
		s := strings.TrimSpace(src.String())
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			dst.Set(reflect.ValueOf(t))
			return nil
		}
		if sec, err := strconv.ParseInt(s, 10, 64); err == nil {
			dst.Set(reflect.ValueOf(time.Unix(sec, 0)))
			return nil
		}
	case reflect.Int, reflect.Int64:
		dst.Set(reflect.ValueOf(time.Unix(src.Int(), 0)))
		return nil
	}

	// 3) Fallback berbasis field (pgtype.*)
	if src.Kind() == reflect.Struct {
		// NULL? (Valid=false) → zero
		if vf := src.FieldByName("Valid"); vf.IsValid() && vf.Kind() == reflect.Bool && !vf.Bool() {
			dst.Set(reflect.Zero(dst.Type()))
			return nil
		}
		// Timestamp/Timestamptz punya field Time time.Time
		if tf := src.FieldByName("Time"); tf.IsValid() && tf.Type() == reflect.TypeOf(time.Time{}) {
			dst.Set(tf)
			return nil
		}
		// Date: Year/Month/Day
		yf, mf, df := src.FieldByName("Year"), src.FieldByName("Month"), src.FieldByName("Day")
		if yf.IsValid() && mf.IsValid() && df.IsValid() &&
			isIntKind(yf.Kind()) && isIntKind(mf.Kind()) && isIntKind(df.Kind()) {
			y := int(yf.Int())
			m := time.Month(mf.Int())
			d := int(df.Int())
			dst.Set(reflect.ValueOf(time.Date(y, m, d, 0, 0, 0, 0, time.UTC)))
			return nil
		}
	}

	return fmt.Errorf("cannot convert %s (%s) to time.Time", src.Kind(), src.Type())
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

func isIntKind(k reflect.Kind) bool {
	return k == reflect.Int || k == reflect.Int8 || k == reflect.Int16 || k == reflect.Int32 || k == reflect.Int64
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
		tmp := structToStringMap(src) // pakai prioritas db > json > Name
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

func structConvert(dst, src reflect.Value) error {
	switch src.Kind() {
	case reflect.Struct:
		srcFields := indexStructFields(src.Type()) // db > json > Name > lowerFirst
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
			b, err := strconv.ParseBool(strings.TrimSpace(src.String()))
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
	return fmt.Errorf("no conversion rule from %s (%s) to %s", src.Kind(), src.Type(), dst.Type())
}
