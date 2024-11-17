// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.35.2
// 	protoc        (unknown)
// source: aksnodeconfig/v1/runc_config.proto

package aksnodeconfigv1

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

type RuncConfig struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The version of runc to use.
	RuncVersion string `protobuf:"bytes,1,opt,name=runc_version,json=runcVersion,proto3" json:"runc_version,omitempty"`
	// The URL to download the runc package from.
	RuncPackageUrl string `protobuf:"bytes,2,opt,name=runc_package_url,json=runcPackageUrl,proto3" json:"runc_package_url,omitempty"`
}

func (x *RuncConfig) Reset() {
	*x = RuncConfig{}
	mi := &file_aksnodeconfig_v1_runc_config_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *RuncConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RuncConfig) ProtoMessage() {}

func (x *RuncConfig) ProtoReflect() protoreflect.Message {
	mi := &file_aksnodeconfig_v1_runc_config_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RuncConfig.ProtoReflect.Descriptor instead.
func (*RuncConfig) Descriptor() ([]byte, []int) {
	return file_aksnodeconfig_v1_runc_config_proto_rawDescGZIP(), []int{0}
}

func (x *RuncConfig) GetRuncVersion() string {
	if x != nil {
		return x.RuncVersion
	}
	return ""
}

func (x *RuncConfig) GetRuncPackageUrl() string {
	if x != nil {
		return x.RuncPackageUrl
	}
	return ""
}

var File_aksnodeconfig_v1_runc_config_proto protoreflect.FileDescriptor

var file_aksnodeconfig_v1_runc_config_proto_rawDesc = []byte{
	0x0a, 0x22, 0x61, 0x6b, 0x73, 0x6e, 0x6f, 0x64, 0x65, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2f,
	0x76, 0x31, 0x2f, 0x72, 0x75, 0x6e, 0x63, 0x5f, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x12, 0x10, 0x61, 0x6b, 0x73, 0x6e, 0x6f, 0x64, 0x65, 0x63, 0x6f, 0x6e,
	0x66, 0x69, 0x67, 0x2e, 0x76, 0x31, 0x22, 0x59, 0x0a, 0x0a, 0x52, 0x75, 0x6e, 0x63, 0x43, 0x6f,
	0x6e, 0x66, 0x69, 0x67, 0x12, 0x21, 0x0a, 0x0c, 0x72, 0x75, 0x6e, 0x63, 0x5f, 0x76, 0x65, 0x72,
	0x73, 0x69, 0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x72, 0x75, 0x6e, 0x63,
	0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x28, 0x0a, 0x10, 0x72, 0x75, 0x6e, 0x63, 0x5f,
	0x70, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x5f, 0x75, 0x72, 0x6c, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x0e, 0x72, 0x75, 0x6e, 0x63, 0x50, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x55, 0x72,
	0x6c, 0x42, 0xca, 0x01, 0x0a, 0x14, 0x63, 0x6f, 0x6d, 0x2e, 0x61, 0x6b, 0x73, 0x6e, 0x6f, 0x64,
	0x65, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x76, 0x31, 0x42, 0x0f, 0x52, 0x75, 0x6e, 0x63,
	0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x40, 0x67,
	0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x41, 0x7a, 0x75, 0x72, 0x65, 0x2f,
	0x41, 0x67, 0x65, 0x6e, 0x74, 0x42, 0x61, 0x6b, 0x65, 0x72, 0x2f, 0x70, 0x6b, 0x67, 0x2f, 0x61,
	0x6b, 0x73, 0x6e, 0x6f, 0x64, 0x65, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2f, 0x76, 0x31, 0x3b,
	0x61, 0x6b, 0x73, 0x6e, 0x6f, 0x64, 0x65, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x76, 0x31, 0xa2,
	0x02, 0x03, 0x41, 0x58, 0x58, 0xaa, 0x02, 0x10, 0x41, 0x6b, 0x73, 0x6e, 0x6f, 0x64, 0x65, 0x63,
	0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x56, 0x31, 0xca, 0x02, 0x10, 0x41, 0x6b, 0x73, 0x6e, 0x6f,
	0x64, 0x65, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x5c, 0x56, 0x31, 0xe2, 0x02, 0x1c, 0x41, 0x6b,
	0x73, 0x6e, 0x6f, 0x64, 0x65, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x5c, 0x56, 0x31, 0x5c, 0x47,
	0x50, 0x42, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0xea, 0x02, 0x11, 0x41, 0x6b, 0x73,
	0x6e, 0x6f, 0x64, 0x65, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x3a, 0x3a, 0x56, 0x31, 0x62, 0x06,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_aksnodeconfig_v1_runc_config_proto_rawDescOnce sync.Once
	file_aksnodeconfig_v1_runc_config_proto_rawDescData = file_aksnodeconfig_v1_runc_config_proto_rawDesc
)

func file_aksnodeconfig_v1_runc_config_proto_rawDescGZIP() []byte {
	file_aksnodeconfig_v1_runc_config_proto_rawDescOnce.Do(func() {
		file_aksnodeconfig_v1_runc_config_proto_rawDescData = protoimpl.X.CompressGZIP(file_aksnodeconfig_v1_runc_config_proto_rawDescData)
	})
	return file_aksnodeconfig_v1_runc_config_proto_rawDescData
}

var file_aksnodeconfig_v1_runc_config_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_aksnodeconfig_v1_runc_config_proto_goTypes = []any{
	(*RuncConfig)(nil), // 0: aksnodeconfig.v1.RuncConfig
}
var file_aksnodeconfig_v1_runc_config_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_aksnodeconfig_v1_runc_config_proto_init() }
func file_aksnodeconfig_v1_runc_config_proto_init() {
	if File_aksnodeconfig_v1_runc_config_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_aksnodeconfig_v1_runc_config_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_aksnodeconfig_v1_runc_config_proto_goTypes,
		DependencyIndexes: file_aksnodeconfig_v1_runc_config_proto_depIdxs,
		MessageInfos:      file_aksnodeconfig_v1_runc_config_proto_msgTypes,
	}.Build()
	File_aksnodeconfig_v1_runc_config_proto = out.File
	file_aksnodeconfig_v1_runc_config_proto_rawDesc = nil
	file_aksnodeconfig_v1_runc_config_proto_goTypes = nil
	file_aksnodeconfig_v1_runc_config_proto_depIdxs = nil
}
