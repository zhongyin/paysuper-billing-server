package pkg

import (
	"fmt"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/paysuper/paysuper-recurring-repository/tools"
	"reflect"
	"strings"
)

const (
	DefaultTagName   = "structure"
	optionOmitempty  = "omitempty"
	optionOmitnested = "omitnested"
	optionString     = "string"
	optionFlatten    = "flatten"
	optionTimestamp  = "timestamp"
)

type Structure struct {
	raw     interface{}
	value   reflect.Value
	TagName string
}

type tagOptions []string

func NewStructureConverter(s interface{}) *Structure {
	return &Structure{
		raw:     s,
		value:   structureValue(s),
		TagName: DefaultTagName,
	}
}

func structureValue(s interface{}) reflect.Value {
	v := reflect.ValueOf(s)

	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		panic("not struct")
	}

	return v
}

func (s *Structure) Map() map[string]interface{} {
	out := make(map[string]interface{})
	s.FillMap(out)
	return out
}

func (s *Structure) FillMap(out map[string]interface{}) {
	if out == nil {
		return
	}

	fields := s.structureFields()

	for _, field := range fields {
		name := field.Name
		val := s.value.FieldByName(name)
		isSubStructure := false
		var finalVal interface{}

		tagName, tagOpts := parseTag(field.Tag.Get(s.TagName))

		if tagName != "" {
			name = tagName
		}

		if tagOpts.Has(optionOmitempty) {
			zero := reflect.Zero(val.Type()).Interface()
			current := val.Interface()

			if reflect.DeepEqual(current, zero) {
				continue
			}
		}

		if !tagOpts.Has(optionOmitnested) {
			finalVal = s.nested(val)

			v := reflect.ValueOf(val.Interface())

			if v.Kind() == reflect.Ptr {
				v = v.Elem()
			}

			switch v.Kind() {
			case reflect.Map, reflect.Struct:
				isSubStructure = true
			}
		} else {
			finalVal = val.Interface()
		}

		if tagOpts.Has(optionString) {
			s, ok := val.Interface().(fmt.Stringer)
			if ok {
				out[name] = s.String()
			}
			continue
		}

		if tagOpts.Has(optionTimestamp) {
			if v, err := ptypes.Timestamp(val.Interface().(*timestamp.Timestamp)); err == nil {
				finalVal = v
			}
		}

		if isSubStructure && (tagOpts.Has(optionFlatten)) {
			for k := range finalVal.(map[string]interface{}) {
				out[k] = finalVal.(map[string]interface{})[k]
			}
		} else {
			out[name] = finalVal
		}
	}
}

func (s *Structure) structureFields() []reflect.StructField {
	t := s.value.Type()

	var f []reflect.StructField

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}

		if tag := field.Tag.Get(s.TagName); tag == "-" {
			continue
		}

		f = append(f, field)
	}

	return f
}

func (s *Structure) nested(val reflect.Value) interface{} {
	var finalVal interface{}

	v := reflect.ValueOf(val.Interface())

	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Struct:
		n := NewStructureConverter(val.Interface())
		n.TagName = s.TagName
		m := n.Map()

		if len(m) == 0 {
			finalVal = fmt.Sprintf("%v", val.Interface())
		} else {
			finalVal = m
		}
	case reflect.Map:
		mapElem := val.Type()

		switch val.Type().Kind() {
		case reflect.Ptr, reflect.Array, reflect.Map,
			reflect.Slice, reflect.Chan:
			mapElem = val.Type().Elem()

			if mapElem.Kind() == reflect.Ptr {
				mapElem = mapElem.Elem()
			}
		}

		if mapElem.Kind() == reflect.Struct ||
			(mapElem.Kind() == reflect.Slice && mapElem.Elem().Kind() == reflect.Struct) {
			m := make(map[string]interface{}, val.Len())

			for _, k := range val.MapKeys() {
				m[k.String()] = s.nested(val.MapIndex(k))
			}

			finalVal = m
			break
		}

		finalVal = val.Interface()
	case reflect.Slice, reflect.Array:
		if val.Type().Kind() == reflect.Interface {
			finalVal = val.Interface()
			break
		}

		if val.Type().Elem().Kind() != reflect.Struct &&
			!(val.Type().Elem().Kind() == reflect.Ptr && val.Type().Elem().Elem().Kind() == reflect.Struct) {
			finalVal = val.Interface()
			break
		}

		slices := make([]interface{}, val.Len())

		for x := 0; x < val.Len(); x++ {
			slices[x] = s.nested(val.Index(x))
		}

		finalVal = slices
	case reflect.Float32, reflect.Float64:
		finalVal = fmt.Sprintf("%v", tools.FormatAmount(val.Interface().(float64)))
		break
	default:
		finalVal = fmt.Sprintf("%v", val.Interface())
	}

	return finalVal
}

func parseTag(tag string) (string, tagOptions) {
	res := strings.Split(tag, ",")
	return res[0], res[1:]
}

func (t tagOptions) Has(opt string) bool {
	for _, tagOpt := range t {
		if tagOpt == opt {
			return true
		}
	}

	return false
}
