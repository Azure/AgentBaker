// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.33.0
// 	protoc        (unknown)
// source: pkg/proto/nbcontract/v1/teleportconfig.proto

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

type TeleportConfig struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The status of the teleportd plugin. If true, the plugin is enabled.
	Status bool `protobuf:"varint,1,opt,name=status,proto3" json:"status,omitempty"`
	// The URL to download the teleportd plugin.
	TeleportdPluginDownloadUrl string `protobuf:"bytes,2,opt,name=teleportd_plugin_download_url,json=teleportdPluginDownloadUrl,proto3" json:"teleportd_plugin_download_url,omitempty"`
}

func (x *TeleportConfig) Reset() {
	*x = TeleportConfig{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_proto_nbcontract_v1_teleportconfig_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TeleportConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TeleportConfig) ProtoMessage() {}

func (x *TeleportConfig) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_proto_nbcontract_v1_teleportconfig_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TeleportConfig.ProtoReflect.Descriptor instead.
func (*TeleportConfig) Descriptor() ([]byte, []int) {
	return file_pkg_proto_nbcontract_v1_teleportconfig_proto_rawDescGZIP(), []int{0}
}

func (x *TeleportConfig) GetStatus() bool {
	if x != nil {
		return x.Status
	}
	return false
}

func (x *TeleportConfig) GetTeleportdPluginDownloadUrl() string {
	if x != nil {
		return x.TeleportdPluginDownloadUrl
	}
	return ""
}

var File_pkg_proto_nbcontract_v1_teleportconfig_proto protoreflect.FileDescriptor

var file_pkg_proto_nbcontract_v1_teleportconfig_proto_rawDesc = []byte{
	0x0a, 0x2c, 0x70, 0x6b, 0x67, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x6e, 0x62, 0x63, 0x6f,
	0x6e, 0x74, 0x72, 0x61, 0x63, 0x74, 0x2f, 0x76, 0x31, 0x2f, 0x74, 0x65, 0x6c, 0x65, 0x70, 0x6f,
	0x72, 0x74, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0d,
	0x6e, 0x62, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63, 0x74, 0x2e, 0x76, 0x31, 0x22, 0x6b, 0x0a,
	0x0e, 0x54, 0x65, 0x6c, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12,
	0x16, 0x0a, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x08, 0x52,
	0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x41, 0x0a, 0x1d, 0x74, 0x65, 0x6c, 0x65, 0x70,
	0x6f, 0x72, 0x74, 0x64, 0x5f, 0x70, 0x6c, 0x75, 0x67, 0x69, 0x6e, 0x5f, 0x64, 0x6f, 0x77, 0x6e,
	0x6c, 0x6f, 0x61, 0x64, 0x5f, 0x75, 0x72, 0x6c, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x1a,
	0x74, 0x65, 0x6c, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x64, 0x50, 0x6c, 0x75, 0x67, 0x69, 0x6e, 0x44,
	0x6f, 0x77, 0x6e, 0x6c, 0x6f, 0x61, 0x64, 0x55, 0x72, 0x6c, 0x42, 0xbf, 0x01, 0x0a, 0x11, 0x63,
	0x6f, 0x6d, 0x2e, 0x6e, 0x62, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63, 0x74, 0x2e, 0x76, 0x31,
	0x42, 0x13, 0x54, 0x65, 0x6c, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67,
	0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x40, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e,
	0x63, 0x6f, 0x6d, 0x2f, 0x41, 0x7a, 0x75, 0x72, 0x65, 0x2f, 0x41, 0x67, 0x65, 0x6e, 0x74, 0x42,
	0x61, 0x6b, 0x65, 0x72, 0x2f, 0x70, 0x6b, 0x67, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x6e,
	0x62, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63, 0x74, 0x2f, 0x76, 0x31, 0x3b, 0x6e, 0x62, 0x63,
	0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63, 0x74, 0x76, 0x31, 0xa2, 0x02, 0x03, 0x4e, 0x58, 0x58, 0xaa,
	0x02, 0x0d, 0x4e, 0x62, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63, 0x74, 0x2e, 0x56, 0x31, 0xca,
	0x02, 0x0d, 0x4e, 0x62, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63, 0x74, 0x5c, 0x56, 0x31, 0xe2,
	0x02, 0x19, 0x4e, 0x62, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63, 0x74, 0x5c, 0x56, 0x31, 0x5c,
	0x47, 0x50, 0x42, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0xea, 0x02, 0x0e, 0x4e, 0x62,
	0x63, 0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63, 0x74, 0x3a, 0x3a, 0x56, 0x31, 0x62, 0x06, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_pkg_proto_nbcontract_v1_teleportconfig_proto_rawDescOnce sync.Once
	file_pkg_proto_nbcontract_v1_teleportconfig_proto_rawDescData = file_pkg_proto_nbcontract_v1_teleportconfig_proto_rawDesc
)

func file_pkg_proto_nbcontract_v1_teleportconfig_proto_rawDescGZIP() []byte {
	file_pkg_proto_nbcontract_v1_teleportconfig_proto_rawDescOnce.Do(func() {
		file_pkg_proto_nbcontract_v1_teleportconfig_proto_rawDescData = protoimpl.X.CompressGZIP(file_pkg_proto_nbcontract_v1_teleportconfig_proto_rawDescData)
	})
	return file_pkg_proto_nbcontract_v1_teleportconfig_proto_rawDescData
}

var file_pkg_proto_nbcontract_v1_teleportconfig_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_pkg_proto_nbcontract_v1_teleportconfig_proto_goTypes = []interface{}{
	(*TeleportConfig)(nil), // 0: nbcontract.v1.TeleportConfig
}
var file_pkg_proto_nbcontract_v1_teleportconfig_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_pkg_proto_nbcontract_v1_teleportconfig_proto_init() }
func file_pkg_proto_nbcontract_v1_teleportconfig_proto_init() {
	if File_pkg_proto_nbcontract_v1_teleportconfig_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_pkg_proto_nbcontract_v1_teleportconfig_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TeleportConfig); i {
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
			RawDescriptor: file_pkg_proto_nbcontract_v1_teleportconfig_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_pkg_proto_nbcontract_v1_teleportconfig_proto_goTypes,
		DependencyIndexes: file_pkg_proto_nbcontract_v1_teleportconfig_proto_depIdxs,
		MessageInfos:      file_pkg_proto_nbcontract_v1_teleportconfig_proto_msgTypes,
	}.Build()
	File_pkg_proto_nbcontract_v1_teleportconfig_proto = out.File
	file_pkg_proto_nbcontract_v1_teleportconfig_proto_rawDesc = nil
	file_pkg_proto_nbcontract_v1_teleportconfig_proto_goTypes = nil
	file_pkg_proto_nbcontract_v1_teleportconfig_proto_depIdxs = nil
}
