// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        (unknown)
// source: pkg/proto/nbcontract/v1/kubebinaryconfig.proto

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

// Kube Binary Config
type KubeBinaryConfig struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// default kube binary url
	KubeBinaryUrl string `protobuf:"bytes,1,opt,name=kube_binary_url,json=kubeBinaryUrl,proto3" json:"kube_binary_url,omitempty"`
	// user's custom kube binary url
	CustomKubeBinaryUrl string `protobuf:"bytes,2,opt,name=custom_kube_binary_url,json=customKubeBinaryUrl,proto3" json:"custom_kube_binary_url,omitempty"`
	// privately cached kube binary url
	PrivateKubeBinaryUrl string `protobuf:"bytes,3,opt,name=private_kube_binary_url,json=privateKubeBinaryUrl,proto3" json:"private_kube_binary_url,omitempty"`
	// full path to the "pause" image. Used for --pod-infra-container-image.
	PodInfraContainerImageUrl string `protobuf:"bytes,4,opt,name=pod_infra_container_image_url,json=podInfraContainerImageUrl,proto3" json:"pod_infra_container_image_url,omitempty"`
	// Full path to the Linux credential provider (tar.gz) to use.
	LinuxCredentialProviderUrl string `protobuf:"bytes,5,opt,name=linux_credential_provider_url,json=linuxCredentialProviderUrl,proto3" json:"linux_credential_provider_url,omitempty"`
}

func (x *KubeBinaryConfig) Reset() {
	*x = KubeBinaryConfig{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_proto_nbcontract_v1_kubebinaryconfig_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *KubeBinaryConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*KubeBinaryConfig) ProtoMessage() {}

func (x *KubeBinaryConfig) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_proto_nbcontract_v1_kubebinaryconfig_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use KubeBinaryConfig.ProtoReflect.Descriptor instead.
func (*KubeBinaryConfig) Descriptor() ([]byte, []int) {
	return file_pkg_proto_nbcontract_v1_kubebinaryconfig_proto_rawDescGZIP(), []int{0}
}

func (x *KubeBinaryConfig) GetKubeBinaryUrl() string {
	if x != nil {
		return x.KubeBinaryUrl
	}
	return ""
}

func (x *KubeBinaryConfig) GetCustomKubeBinaryUrl() string {
	if x != nil {
		return x.CustomKubeBinaryUrl
	}
	return ""
}

func (x *KubeBinaryConfig) GetPrivateKubeBinaryUrl() string {
	if x != nil {
		return x.PrivateKubeBinaryUrl
	}
	return ""
}

func (x *KubeBinaryConfig) GetPodInfraContainerImageUrl() string {
	if x != nil {
		return x.PodInfraContainerImageUrl
	}
	return ""
}

func (x *KubeBinaryConfig) GetLinuxCredentialProviderUrl() string {
	if x != nil {
		return x.LinuxCredentialProviderUrl
	}
	return ""
}

var File_pkg_proto_nbcontract_v1_kubebinaryconfig_proto protoreflect.FileDescriptor

var file_pkg_proto_nbcontract_v1_kubebinaryconfig_proto_rawDesc = []byte{
	0x0a, 0x2e, 0x70, 0x6b, 0x67, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x6e, 0x62, 0x63, 0x6f,
	0x6e, 0x74, 0x72, 0x61, 0x63, 0x74, 0x2f, 0x76, 0x31, 0x2f, 0x6b, 0x75, 0x62, 0x65, 0x62, 0x69,
	0x6e, 0x61, 0x72, 0x79, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x12, 0x0d, 0x6e, 0x62, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63, 0x74, 0x2e, 0x76, 0x31, 0x22,
	0xab, 0x02, 0x0a, 0x10, 0x4b, 0x75, 0x62, 0x65, 0x42, 0x69, 0x6e, 0x61, 0x72, 0x79, 0x43, 0x6f,
	0x6e, 0x66, 0x69, 0x67, 0x12, 0x26, 0x0a, 0x0f, 0x6b, 0x75, 0x62, 0x65, 0x5f, 0x62, 0x69, 0x6e,
	0x61, 0x72, 0x79, 0x5f, 0x75, 0x72, 0x6c, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0d, 0x6b,
	0x75, 0x62, 0x65, 0x42, 0x69, 0x6e, 0x61, 0x72, 0x79, 0x55, 0x72, 0x6c, 0x12, 0x33, 0x0a, 0x16,
	0x63, 0x75, 0x73, 0x74, 0x6f, 0x6d, 0x5f, 0x6b, 0x75, 0x62, 0x65, 0x5f, 0x62, 0x69, 0x6e, 0x61,
	0x72, 0x79, 0x5f, 0x75, 0x72, 0x6c, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x13, 0x63, 0x75,
	0x73, 0x74, 0x6f, 0x6d, 0x4b, 0x75, 0x62, 0x65, 0x42, 0x69, 0x6e, 0x61, 0x72, 0x79, 0x55, 0x72,
	0x6c, 0x12, 0x35, 0x0a, 0x17, 0x70, 0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x5f, 0x6b, 0x75, 0x62,
	0x65, 0x5f, 0x62, 0x69, 0x6e, 0x61, 0x72, 0x79, 0x5f, 0x75, 0x72, 0x6c, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x14, 0x70, 0x72, 0x69, 0x76, 0x61, 0x74, 0x65, 0x4b, 0x75, 0x62, 0x65, 0x42,
	0x69, 0x6e, 0x61, 0x72, 0x79, 0x55, 0x72, 0x6c, 0x12, 0x40, 0x0a, 0x1d, 0x70, 0x6f, 0x64, 0x5f,
	0x69, 0x6e, 0x66, 0x72, 0x61, 0x5f, 0x63, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x5f,
	0x69, 0x6d, 0x61, 0x67, 0x65, 0x5f, 0x75, 0x72, 0x6c, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x19, 0x70, 0x6f, 0x64, 0x49, 0x6e, 0x66, 0x72, 0x61, 0x43, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e,
	0x65, 0x72, 0x49, 0x6d, 0x61, 0x67, 0x65, 0x55, 0x72, 0x6c, 0x12, 0x41, 0x0a, 0x1d, 0x6c, 0x69,
	0x6e, 0x75, 0x78, 0x5f, 0x63, 0x72, 0x65, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x61, 0x6c, 0x5f, 0x70,
	0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72, 0x5f, 0x75, 0x72, 0x6c, 0x18, 0x05, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x1a, 0x6c, 0x69, 0x6e, 0x75, 0x78, 0x43, 0x72, 0x65, 0x64, 0x65, 0x6e, 0x74, 0x69,
	0x61, 0x6c, 0x50, 0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72, 0x55, 0x72, 0x6c, 0x42, 0xc1, 0x01,
	0x0a, 0x11, 0x63, 0x6f, 0x6d, 0x2e, 0x6e, 0x62, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63, 0x74,
	0x2e, 0x76, 0x31, 0x42, 0x15, 0x4b, 0x75, 0x62, 0x65, 0x62, 0x69, 0x6e, 0x61, 0x72, 0x79, 0x63,
	0x6f, 0x6e, 0x66, 0x69, 0x67, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x40, 0x67, 0x69,
	0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x41, 0x7a, 0x75, 0x72, 0x65, 0x2f, 0x41,
	0x67, 0x65, 0x6e, 0x74, 0x42, 0x61, 0x6b, 0x65, 0x72, 0x2f, 0x70, 0x6b, 0x67, 0x2f, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x2f, 0x6e, 0x62, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63, 0x74, 0x2f, 0x76,
	0x31, 0x3b, 0x6e, 0x62, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63, 0x74, 0x76, 0x31, 0xa2, 0x02,
	0x03, 0x4e, 0x58, 0x58, 0xaa, 0x02, 0x0d, 0x4e, 0x62, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63,
	0x74, 0x2e, 0x56, 0x31, 0xca, 0x02, 0x0d, 0x4e, 0x62, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63,
	0x74, 0x5c, 0x56, 0x31, 0xe2, 0x02, 0x19, 0x4e, 0x62, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63,
	0x74, 0x5c, 0x56, 0x31, 0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61,
	0xea, 0x02, 0x0e, 0x4e, 0x62, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63, 0x74, 0x3a, 0x3a, 0x56,
	0x31, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_pkg_proto_nbcontract_v1_kubebinaryconfig_proto_rawDescOnce sync.Once
	file_pkg_proto_nbcontract_v1_kubebinaryconfig_proto_rawDescData = file_pkg_proto_nbcontract_v1_kubebinaryconfig_proto_rawDesc
)

func file_pkg_proto_nbcontract_v1_kubebinaryconfig_proto_rawDescGZIP() []byte {
	file_pkg_proto_nbcontract_v1_kubebinaryconfig_proto_rawDescOnce.Do(func() {
		file_pkg_proto_nbcontract_v1_kubebinaryconfig_proto_rawDescData = protoimpl.X.CompressGZIP(file_pkg_proto_nbcontract_v1_kubebinaryconfig_proto_rawDescData)
	})
	return file_pkg_proto_nbcontract_v1_kubebinaryconfig_proto_rawDescData
}

var file_pkg_proto_nbcontract_v1_kubebinaryconfig_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_pkg_proto_nbcontract_v1_kubebinaryconfig_proto_goTypes = []any{
	(*KubeBinaryConfig)(nil), // 0: nbcontract.v1.KubeBinaryConfig
}
var file_pkg_proto_nbcontract_v1_kubebinaryconfig_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_pkg_proto_nbcontract_v1_kubebinaryconfig_proto_init() }
func file_pkg_proto_nbcontract_v1_kubebinaryconfig_proto_init() {
	if File_pkg_proto_nbcontract_v1_kubebinaryconfig_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_pkg_proto_nbcontract_v1_kubebinaryconfig_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*KubeBinaryConfig); i {
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
			RawDescriptor: file_pkg_proto_nbcontract_v1_kubebinaryconfig_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_pkg_proto_nbcontract_v1_kubebinaryconfig_proto_goTypes,
		DependencyIndexes: file_pkg_proto_nbcontract_v1_kubebinaryconfig_proto_depIdxs,
		MessageInfos:      file_pkg_proto_nbcontract_v1_kubebinaryconfig_proto_msgTypes,
	}.Build()
	File_pkg_proto_nbcontract_v1_kubebinaryconfig_proto = out.File
	file_pkg_proto_nbcontract_v1_kubebinaryconfig_proto_rawDesc = nil
	file_pkg_proto_nbcontract_v1_kubebinaryconfig_proto_goTypes = nil
	file_pkg_proto_nbcontract_v1_kubebinaryconfig_proto_depIdxs = nil
}
