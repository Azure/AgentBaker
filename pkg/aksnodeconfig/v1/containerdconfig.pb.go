// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.35.2
// 	protoc        (unknown)
// source: aksnodeconfig/v1/containerdconfig.proto

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

type ContainerdConfig struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The base URL for downloading containerd.
	ContainerdDownloadUrlBase string `protobuf:"bytes,1,opt,name=containerd_download_url_base,json=containerdDownloadUrlBase,proto3" json:"containerd_download_url_base,omitempty"`
	// The version of containerd to download.
	ContainerdVersion string `protobuf:"bytes,2,opt,name=containerd_version,json=containerdVersion,proto3" json:"containerd_version,omitempty"`
	// The URL for downloading the containerd package.
	ContainerdPackageUrl string `protobuf:"bytes,3,opt,name=containerd_package_url,json=containerdPackageUrl,proto3" json:"containerd_package_url,omitempty"`
}

func (x *ContainerdConfig) Reset() {
	*x = ContainerdConfig{}
	mi := &file_aksnodeconfig_v1_containerdconfig_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ContainerdConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ContainerdConfig) ProtoMessage() {}

func (x *ContainerdConfig) ProtoReflect() protoreflect.Message {
	mi := &file_aksnodeconfig_v1_containerdconfig_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ContainerdConfig.ProtoReflect.Descriptor instead.
func (*ContainerdConfig) Descriptor() ([]byte, []int) {
	return file_aksnodeconfig_v1_containerdconfig_proto_rawDescGZIP(), []int{0}
}

func (x *ContainerdConfig) GetContainerdDownloadUrlBase() string {
	if x != nil {
		return x.ContainerdDownloadUrlBase
	}
	return ""
}

func (x *ContainerdConfig) GetContainerdVersion() string {
	if x != nil {
		return x.ContainerdVersion
	}
	return ""
}

func (x *ContainerdConfig) GetContainerdPackageUrl() string {
	if x != nil {
		return x.ContainerdPackageUrl
	}
	return ""
}

var File_aksnodeconfig_v1_containerdconfig_proto protoreflect.FileDescriptor

var file_aksnodeconfig_v1_containerdconfig_proto_rawDesc = []byte{
	0x0a, 0x27, 0x61, 0x6b, 0x73, 0x6e, 0x6f, 0x64, 0x65, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2f,
	0x76, 0x31, 0x2f, 0x63, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x64, 0x63, 0x6f, 0x6e,
	0x66, 0x69, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x10, 0x61, 0x6b, 0x73, 0x6e, 0x6f,
	0x64, 0x65, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x76, 0x31, 0x22, 0xb8, 0x01, 0x0a, 0x10,
	0x43, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x64, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67,
	0x12, 0x3f, 0x0a, 0x1c, 0x63, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x64, 0x5f, 0x64,
	0x6f, 0x77, 0x6e, 0x6c, 0x6f, 0x61, 0x64, 0x5f, 0x75, 0x72, 0x6c, 0x5f, 0x62, 0x61, 0x73, 0x65,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x19, 0x63, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65,
	0x72, 0x64, 0x44, 0x6f, 0x77, 0x6e, 0x6c, 0x6f, 0x61, 0x64, 0x55, 0x72, 0x6c, 0x42, 0x61, 0x73,
	0x65, 0x12, 0x2d, 0x0a, 0x12, 0x63, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x64, 0x5f,
	0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x11, 0x63,
	0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x64, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e,
	0x12, 0x34, 0x0a, 0x16, 0x63, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x64, 0x5f, 0x70,
	0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x5f, 0x75, 0x72, 0x6c, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x14, 0x63, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x64, 0x50, 0x61, 0x63, 0x6b,
	0x61, 0x67, 0x65, 0x55, 0x72, 0x6c, 0x42, 0xd0, 0x01, 0x0a, 0x14, 0x63, 0x6f, 0x6d, 0x2e, 0x61,
	0x6b, 0x73, 0x6e, 0x6f, 0x64, 0x65, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x76, 0x31, 0x42,
	0x15, 0x43, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x64, 0x63, 0x6f, 0x6e, 0x66, 0x69,
	0x67, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x40, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62,
	0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x41, 0x7a, 0x75, 0x72, 0x65, 0x2f, 0x41, 0x67, 0x65, 0x6e, 0x74,
	0x42, 0x61, 0x6b, 0x65, 0x72, 0x2f, 0x70, 0x6b, 0x67, 0x2f, 0x61, 0x6b, 0x73, 0x6e, 0x6f, 0x64,
	0x65, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2f, 0x76, 0x31, 0x3b, 0x61, 0x6b, 0x73, 0x6e, 0x6f,
	0x64, 0x65, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x76, 0x31, 0xa2, 0x02, 0x03, 0x41, 0x58, 0x58,
	0xaa, 0x02, 0x10, 0x41, 0x6b, 0x73, 0x6e, 0x6f, 0x64, 0x65, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67,
	0x2e, 0x56, 0x31, 0xca, 0x02, 0x10, 0x41, 0x6b, 0x73, 0x6e, 0x6f, 0x64, 0x65, 0x63, 0x6f, 0x6e,
	0x66, 0x69, 0x67, 0x5c, 0x56, 0x31, 0xe2, 0x02, 0x1c, 0x41, 0x6b, 0x73, 0x6e, 0x6f, 0x64, 0x65,
	0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x5c, 0x56, 0x31, 0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65, 0x74,
	0x61, 0x64, 0x61, 0x74, 0x61, 0xea, 0x02, 0x11, 0x41, 0x6b, 0x73, 0x6e, 0x6f, 0x64, 0x65, 0x63,
	0x6f, 0x6e, 0x66, 0x69, 0x67, 0x3a, 0x3a, 0x56, 0x31, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x33,
}

var (
	file_aksnodeconfig_v1_containerdconfig_proto_rawDescOnce sync.Once
	file_aksnodeconfig_v1_containerdconfig_proto_rawDescData = file_aksnodeconfig_v1_containerdconfig_proto_rawDesc
)

func file_aksnodeconfig_v1_containerdconfig_proto_rawDescGZIP() []byte {
	file_aksnodeconfig_v1_containerdconfig_proto_rawDescOnce.Do(func() {
		file_aksnodeconfig_v1_containerdconfig_proto_rawDescData = protoimpl.X.CompressGZIP(file_aksnodeconfig_v1_containerdconfig_proto_rawDescData)
	})
	return file_aksnodeconfig_v1_containerdconfig_proto_rawDescData
}

var file_aksnodeconfig_v1_containerdconfig_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_aksnodeconfig_v1_containerdconfig_proto_goTypes = []any{
	(*ContainerdConfig)(nil), // 0: aksnodeconfig.v1.ContainerdConfig
}
var file_aksnodeconfig_v1_containerdconfig_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_aksnodeconfig_v1_containerdconfig_proto_init() }
func file_aksnodeconfig_v1_containerdconfig_proto_init() {
	if File_aksnodeconfig_v1_containerdconfig_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_aksnodeconfig_v1_containerdconfig_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_aksnodeconfig_v1_containerdconfig_proto_goTypes,
		DependencyIndexes: file_aksnodeconfig_v1_containerdconfig_proto_depIdxs,
		MessageInfos:      file_aksnodeconfig_v1_containerdconfig_proto_msgTypes,
	}.Build()
	File_aksnodeconfig_v1_containerdconfig_proto = out.File
	file_aksnodeconfig_v1_containerdconfig_proto_rawDesc = nil
	file_aksnodeconfig_v1_containerdconfig_proto_goTypes = nil
	file_aksnodeconfig_v1_containerdconfig_proto_depIdxs = nil
}
