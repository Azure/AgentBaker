// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        (unknown)
// source: pkg/proto/nbcontract/v1/customsearchdomainconfig.proto

package nbcontractv1

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type CustomSearchDomainConfig struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The name of the custom search domain.
	DomainName string `protobuf:"bytes,1,opt,name=domain_name,json=domainName,proto3" json:"domain_name,omitempty"`
	// The user name for the custom search domain.
	RealmUser string `protobuf:"bytes,2,opt,name=realm_user,json=realmUser,proto3" json:"realm_user,omitempty"`
	// The password for the custom search domain.
	RealmPassword string `protobuf:"bytes,3,opt,name=realm_password,json=realmPassword,proto3" json:"realm_password,omitempty"`
}

func (x *CustomSearchDomainConfig) Reset() {
	*x = CustomSearchDomainConfig{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_proto_nbcontract_v1_customsearchdomainconfig_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CustomSearchDomainConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CustomSearchDomainConfig) ProtoMessage() {}

func (x *CustomSearchDomainConfig) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_proto_nbcontract_v1_customsearchdomainconfig_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CustomSearchDomainConfig.ProtoReflect.Descriptor instead.
func (*CustomSearchDomainConfig) Descriptor() ([]byte, []int) {
	return file_pkg_proto_nbcontract_v1_customsearchdomainconfig_proto_rawDescGZIP(), []int{0}
}

func (x *CustomSearchDomainConfig) GetDomainName() string {
	if x != nil {
		return x.DomainName
	}
	return ""
}

func (x *CustomSearchDomainConfig) GetRealmUser() string {
	if x != nil {
		return x.RealmUser
	}
	return ""
}

func (x *CustomSearchDomainConfig) GetRealmPassword() string {
	if x != nil {
		return x.RealmPassword
	}
	return ""
}

var File_pkg_proto_nbcontract_v1_customsearchdomainconfig_proto protoreflect.FileDescriptor

var file_pkg_proto_nbcontract_v1_customsearchdomainconfig_proto_rawDesc = []byte{
	0x0a, 0x36, 0x70, 0x6b, 0x67, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x6e, 0x62, 0x63, 0x6f,
	0x6e, 0x74, 0x72, 0x61, 0x63, 0x74, 0x2f, 0x76, 0x31, 0x2f, 0x63, 0x75, 0x73, 0x74, 0x6f, 0x6d,
	0x73, 0x65, 0x61, 0x72, 0x63, 0x68, 0x64, 0x6f, 0x6d, 0x61, 0x69, 0x6e, 0x63, 0x6f, 0x6e, 0x66,
	0x69, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0d, 0x6e, 0x62, 0x63, 0x6f, 0x6e, 0x74,
	0x72, 0x61, 0x63, 0x74, 0x2e, 0x76, 0x31, 0x22, 0x81, 0x01, 0x0a, 0x18, 0x43, 0x75, 0x73, 0x74,
	0x6f, 0x6d, 0x53, 0x65, 0x61, 0x72, 0x63, 0x68, 0x44, 0x6f, 0x6d, 0x61, 0x69, 0x6e, 0x43, 0x6f,
	0x6e, 0x66, 0x69, 0x67, 0x12, 0x1f, 0x0a, 0x0b, 0x64, 0x6f, 0x6d, 0x61, 0x69, 0x6e, 0x5f, 0x6e,
	0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x64, 0x6f, 0x6d, 0x61, 0x69,
	0x6e, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x1d, 0x0a, 0x0a, 0x72, 0x65, 0x61, 0x6c, 0x6d, 0x5f, 0x75,
	0x73, 0x65, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x72, 0x65, 0x61, 0x6c, 0x6d,
	0x55, 0x73, 0x65, 0x72, 0x12, 0x25, 0x0a, 0x0e, 0x72, 0x65, 0x61, 0x6c, 0x6d, 0x5f, 0x70, 0x61,
	0x73, 0x73, 0x77, 0x6f, 0x72, 0x64, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0d, 0x72, 0x65,
	0x61, 0x6c, 0x6d, 0x50, 0x61, 0x73, 0x73, 0x77, 0x6f, 0x72, 0x64, 0x42, 0xc9, 0x01, 0x0a, 0x11,
	0x63, 0x6f, 0x6d, 0x2e, 0x6e, 0x62, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63, 0x74, 0x2e, 0x76,
	0x31, 0x42, 0x1d, 0x43, 0x75, 0x73, 0x74, 0x6f, 0x6d, 0x73, 0x65, 0x61, 0x72, 0x63, 0x68, 0x64,
	0x6f, 0x6d, 0x61, 0x69, 0x6e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x50, 0x72, 0x6f, 0x74, 0x6f,
	0x50, 0x01, 0x5a, 0x40, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x41,
	0x7a, 0x75, 0x72, 0x65, 0x2f, 0x41, 0x67, 0x65, 0x6e, 0x74, 0x42, 0x61, 0x6b, 0x65, 0x72, 0x2f,
	0x70, 0x6b, 0x67, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x6e, 0x62, 0x63, 0x6f, 0x6e, 0x74,
	0x72, 0x61, 0x63, 0x74, 0x2f, 0x76, 0x31, 0x3b, 0x6e, 0x62, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x61,
	0x63, 0x74, 0x76, 0x31, 0xa2, 0x02, 0x03, 0x4e, 0x58, 0x58, 0xaa, 0x02, 0x0d, 0x4e, 0x62, 0x63,
	0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63, 0x74, 0x2e, 0x56, 0x31, 0xca, 0x02, 0x0d, 0x4e, 0x62, 0x63,
	0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63, 0x74, 0x5c, 0x56, 0x31, 0xe2, 0x02, 0x19, 0x4e, 0x62, 0x63,
	0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63, 0x74, 0x5c, 0x56, 0x31, 0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65,
	0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0xea, 0x02, 0x0e, 0x4e, 0x62, 0x63, 0x6f, 0x6e, 0x74, 0x72,
	0x61, 0x63, 0x74, 0x3a, 0x3a, 0x56, 0x31, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_pkg_proto_nbcontract_v1_customsearchdomainconfig_proto_rawDescOnce sync.Once
	file_pkg_proto_nbcontract_v1_customsearchdomainconfig_proto_rawDescData = file_pkg_proto_nbcontract_v1_customsearchdomainconfig_proto_rawDesc
)

func file_pkg_proto_nbcontract_v1_customsearchdomainconfig_proto_rawDescGZIP() []byte {
	file_pkg_proto_nbcontract_v1_customsearchdomainconfig_proto_rawDescOnce.Do(func() {
		file_pkg_proto_nbcontract_v1_customsearchdomainconfig_proto_rawDescData = protoimpl.X.CompressGZIP(file_pkg_proto_nbcontract_v1_customsearchdomainconfig_proto_rawDescData)
	})
	return file_pkg_proto_nbcontract_v1_customsearchdomainconfig_proto_rawDescData
}

var file_pkg_proto_nbcontract_v1_customsearchdomainconfig_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_pkg_proto_nbcontract_v1_customsearchdomainconfig_proto_goTypes = []any{
	(*CustomSearchDomainConfig)(nil), // 0: nbcontract.v1.CustomSearchDomainConfig
}
var file_pkg_proto_nbcontract_v1_customsearchdomainconfig_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_pkg_proto_nbcontract_v1_customsearchdomainconfig_proto_init() }
func file_pkg_proto_nbcontract_v1_customsearchdomainconfig_proto_init() {
	if File_pkg_proto_nbcontract_v1_customsearchdomainconfig_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_pkg_proto_nbcontract_v1_customsearchdomainconfig_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*CustomSearchDomainConfig); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_pkg_proto_nbcontract_v1_customsearchdomainconfig_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_pkg_proto_nbcontract_v1_customsearchdomainconfig_proto_goTypes,
		DependencyIndexes: file_pkg_proto_nbcontract_v1_customsearchdomainconfig_proto_depIdxs,
		MessageInfos:      file_pkg_proto_nbcontract_v1_customsearchdomainconfig_proto_msgTypes,
	}.Build()
	File_pkg_proto_nbcontract_v1_customsearchdomainconfig_proto = out.File
	file_pkg_proto_nbcontract_v1_customsearchdomainconfig_proto_rawDesc = nil
	file_pkg_proto_nbcontract_v1_customsearchdomainconfig_proto_goTypes = nil
	file_pkg_proto_nbcontract_v1_customsearchdomainconfig_proto_depIdxs = nil
}
