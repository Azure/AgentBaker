package nodeconfigutils

import (
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// PopulateAllFields recursively populates all fields in a protobuf message with non-zero test values.
// This is useful for testing to ensure all fields can be marshaled/unmarshaled correctly.
func PopulateAllFields(msg proto.Message) {
	populateMessage(msg.ProtoReflect(), 0)
}

func populateMessage(msg protoreflect.Message, depth int) {
	// Prevent infinite recursion for deeply nested structures
	if depth > 10 {
		return
	}

	// Iterate over all field descriptors (including unset ones)
	fields := msg.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		setFieldValue(msg, fd, depth)
	}
}

func setFieldValue(msg protoreflect.Message, fd protoreflect.FieldDescriptor, depth int) {
	if fd.IsList() {
		// Handle repeated fields - add 2 elements
		list := msg.Mutable(fd).List()
		for j := 0; j < 2; j++ {
			if fd.Kind() == protoreflect.MessageKind {
				// For repeated message fields, create new message and populate it
				elem := list.NewElement()
				populateMessage(elem.Message(), depth+1)
				list.Append(elem)
			} else {
				val := getDefaultValueForField(fd, fmt.Sprintf("item%d", j), depth)
				list.Append(val)
			}
		}
	} else if fd.IsMap() {
		// Handle map fields - add 2 entries
		mapVal := msg.Mutable(fd).Map()
		for j := 0; j < 2; j++ {
			key := getDefaultKeyForKind(fd.MapKey().Kind(), j)
			if fd.MapValue().Kind() == protoreflect.MessageKind {
				// For map values that are messages, create and populate
				val := mapVal.NewValue()
				populateMessage(val.Message(), depth+1)
				mapVal.Set(key, val)
			} else {
				val := getDefaultValueForMapValue(fd.MapValue(), j, depth)
				mapVal.Set(key, val)
			}
		}
	} else if fd.Kind() == protoreflect.MessageKind {
		// Handle singular message fields - use Mutable to get/create the message
		nestedMsg := msg.Mutable(fd).Message()
		populateMessage(nestedMsg, depth+1)
	} else {
		// Handle singular primitive fields
		val := getDefaultValueForField(fd, "", depth)
		msg.Set(fd, val)
	}
}

func getDefaultValueForField(fd protoreflect.FieldDescriptor, suffix string, depth int) protoreflect.Value {
	switch fd.Kind() {
	case protoreflect.BoolKind:
		return protoreflect.ValueOfBool(true)
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return protoreflect.ValueOfInt32(42)
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return protoreflect.ValueOfInt64(42)
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return protoreflect.ValueOfUint32(42)
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return protoreflect.ValueOfUint64(42)
	case protoreflect.FloatKind:
		return protoreflect.ValueOfFloat32(42.0)
	case protoreflect.DoubleKind:
		return protoreflect.ValueOfFloat64(42.0)
	case protoreflect.StringKind:
		fieldName := string(fd.Name())
		if suffix != "" {
			return protoreflect.ValueOfString(fmt.Sprintf("test-%s-%s", fieldName, suffix))
		}
		return protoreflect.ValueOfString(fmt.Sprintf("test-%s-value", fieldName))
	case protoreflect.BytesKind:
		return protoreflect.ValueOfBytes([]byte(fmt.Sprintf("test-bytes-%s", fd.Name())))
	case protoreflect.EnumKind:
		// Use the last enum value (latest/most recent)
		enumDesc := fd.Enum()
		lastIndex := enumDesc.Values().Len() - 1
		if lastIndex >= 0 {
			return protoreflect.ValueOfEnum(enumDesc.Values().Get(lastIndex).Number())
		}
		return protoreflect.ValueOfEnum(0)
	case protoreflect.MessageKind:
		// Message fields should be handled in setFieldValue using Mutable
		// This shouldn't be called for message fields
		return protoreflect.Value{}
	default:
		return protoreflect.Value{}
	}
}

func getDefaultKeyForKind(kind protoreflect.Kind, index int) protoreflect.MapKey {
	switch kind {
	case protoreflect.StringKind:
		return protoreflect.ValueOfString(fmt.Sprintf("test-key-%d", index)).MapKey()
	case protoreflect.Int32Kind:
		return protoreflect.ValueOfInt32(int32(index + 1)).MapKey()
	case protoreflect.Int64Kind:
		return protoreflect.ValueOfInt64(int64(index + 1)).MapKey()
	case protoreflect.Uint32Kind:
		return protoreflect.ValueOfUint32(uint32(index + 1)).MapKey()
	case protoreflect.Uint64Kind:
		return protoreflect.ValueOfUint64(uint64(index + 1)).MapKey()
	case protoreflect.BoolKind:
		return protoreflect.ValueOfBool(index == 0).MapKey()
	default:
		return protoreflect.ValueOfString(fmt.Sprintf("key-%d", index)).MapKey()
	}
}

func getDefaultValueForMapValue(fd protoreflect.FieldDescriptor, index int, depth int) protoreflect.Value {
	suffix := fmt.Sprintf("map%d", index)
	return getDefaultValueForField(fd, suffix, depth)
}
