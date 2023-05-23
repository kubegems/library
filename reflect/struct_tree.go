package reflect

import (
	"reflect"
	"strconv"
	"strings"
)

type Node struct {
	Name     string
	Kind     reflect.Kind
	Tag      reflect.StructTag
	Value    reflect.Value
	Children []Node
}

func ParseStruct(data interface{}) Node {
	v := reflect.Indirect(reflect.ValueOf(data))
	return decode(Node{}, v)
}

func ToJsonPathes(prefix string, nodes []Node) []KV {
	return toJsonPathes(prefix, nodes, []KV{})
}

func prefixedKey(prefix, key string, splitor ...string) string {
	if len(prefix) == 0 {
		return strings.ToLower(key)
	}

	spl := "-"
	if len(splitor) > 0 {
		spl = string(splitor[0])
	}
	return strings.ToLower(prefix + spl + key)
}

type KV struct {
	Key   string
	Value interface{}
}

func toJsonPathes(prefix string, nodes []Node, kvs []KV) []KV {
	for _, node := range nodes {
		switch node.Kind {
		case reflect.Struct, reflect.Map:
			kvs = toJsonPathes(prefixedKey(prefix, node.Name, "."), node.Children, kvs)
		default:
			kvs = append(kvs, KV{
				Key:   prefixedKey(prefix, node.Name, "."),
				Value: node.Value.Interface(),
			})
		}
	}
	return kvs
}

func decode(node Node, v reflect.Value) Node {
	v = reflect.Indirect(v)

	node.Kind = v.Kind()
	node.Value = v

	var children []Node
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			fi := v.Type().Field(i)

			// unexported
			if fi.PkgPath != "" {
				continue
			}

			opts := fi.Tag.Get("json")
			if opts == "" {
				opts = fi.Tag.Get("yaml")
			}

			jsonopts := strings.Split(opts, ",")

			if fi.Anonymous || (len(jsonopts) > 1 && jsonopts[1] == "inline") {
				children = append(children, decode(Node{}, v.Field(i)).Children...)
				continue
			}

			name := jsonopts[0]
			if name == "" {
				name = fi.Name
			}
			in := Node{
				Name: name,
				Tag:  fi.Tag,
			}
			children = append(children, decode(in, v.Field(i)))
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			in := Node{
				Name: k.String(),
			}
			children = append(children, decode(in, v.MapIndex(k)))
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			in := Node{
				Name: strconv.Itoa(i),
			}
			children = append(children, decode(in, v.Index(i)))
		}
	}

	node.Children = children
	return node
}
