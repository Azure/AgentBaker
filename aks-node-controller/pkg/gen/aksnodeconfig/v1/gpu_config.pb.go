// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.35.2
// 	protoc        v5.28.3
// source: aksnodeconfig/v1/gpu_config.proto

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

type GpuConfig struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Specifies whether any nvidia configurations should be applied for GPU nodes. False when vm size is not a GPU node or driver install is skipped and no GPU configuration is needed.
	// Making optional so that default will be set to IsNvidiaEnabledSku(vmSize) instead of false
	EnableNvidia *bool `protobuf:"varint,1,opt,name=enable_nvidia,json=enableNvidia,proto3,oneof" json:"enable_nvidia,omitempty"`
	// Specifies whether bootstrap process should install and configure the GPU driver when necessary. Configuration includes appropriate set up of components like the fabric manager where applicable.
	ConfigGpuDriver bool `protobuf:"varint,2,opt,name=config_gpu_driver,json=configGpuDriver,proto3" json:"config_gpu_driver,omitempty"`
	// Specifies whether special config is needed for MIG GPUs that use GPU dedicated VHDs and enable the device plugin (for all GPU dedicated VHDs)
	GpuDevicePlugin bool `protobuf:"varint,3,opt,name=gpu_device_plugin,json=gpuDevicePlugin,proto3" json:"gpu_device_plugin,omitempty"`
	// Represents the GPU instance profile.
	GpuInstanceProfile string `protobuf:"bytes,4,opt,name=gpu_instance_profile,json=gpuInstanceProfile,proto3" json:"gpu_instance_profile,omitempty"`
	// Same as enable_nvidia, but for AMD GPUs.
	EnableAmdGpu *bool `protobuf:"varint,5,opt,name=enable_amd_gpu,json=enableAmdGpu,proto3,oneof" json:"enable_amd_gpu,omitempty"`
}

func (x *GpuConfig) Reset() {
	*x = GpuConfig{}
	mi := &file_aksnodeconfig_v1_gpu_config_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GpuConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GpuConfig) ProtoMessage() {}

func (x *GpuConfig) ProtoReflect() protoreflect.Message {
	mi := &file_aksnodeconfig_v1_gpu_config_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GpuConfig.ProtoReflect.Descriptor instead.
func (*GpuConfig) Descriptor() ([]byte, []int) {
	return file_aksnodeconfig_v1_gpu_config_proto_rawDescGZIP(), []int{0}
}

func (x *GpuConfig) GetEnableNvidia() bool {
	if x != nil && x.EnableNvidia != nil {
		return *x.EnableNvidia
	}
	return false
}

func (x *GpuConfig) GetConfigGpuDriver() bool {
	if x != nil {
		return x.ConfigGpuDriver
	}
	return false
}

func (x *GpuConfig) GetGpuDevicePlugin() bool {
	if x != nil {
		return x.GpuDevicePlugin
	}
	return false
}

func (x *GpuConfig) GetGpuInstanceProfile() string {
	if x != nil {
		return x.GpuInstanceProfile
	}
	return ""
}

func (x *GpuConfig) GetEnableAmdGpu() bool {
	if x != nil && x.EnableAmdGpu != nil {
		return *x.EnableAmdGpu
	}
	return false
}

var File_aksnodeconfig_v1_gpu_config_proto protoreflect.FileDescriptor

var file_aksnodeconfig_v1_gpu_config_proto_rawDesc = []byte{
	0x0a, 0x21, 0x61, 0x6b, 0x73, 0x6e, 0x6f, 0x64, 0x65, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2f,
	0x76, 0x31, 0x2f, 0x67, 0x70, 0x75, 0x5f, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x12, 0x10, 0x61, 0x6b, 0x73, 0x6e, 0x6f, 0x64, 0x65, 0x63, 0x6f, 0x6e, 0x66,
	0x69, 0x67, 0x2e, 0x76, 0x31, 0x22, 0x8f, 0x02, 0x0a, 0x09, 0x47, 0x70, 0x75, 0x43, 0x6f, 0x6e,
	0x66, 0x69, 0x67, 0x12, 0x28, 0x0a, 0x0d, 0x65, 0x6e, 0x61, 0x62, 0x6c, 0x65, 0x5f, 0x6e, 0x76,
	0x69, 0x64, 0x69, 0x61, 0x18, 0x01, 0x20, 0x01, 0x28, 0x08, 0x48, 0x00, 0x52, 0x0c, 0x65, 0x6e,
	0x61, 0x62, 0x6c, 0x65, 0x4e, 0x76, 0x69, 0x64, 0x69, 0x61, 0x88, 0x01, 0x01, 0x12, 0x2a, 0x0a,
	0x11, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x5f, 0x67, 0x70, 0x75, 0x5f, 0x64, 0x72, 0x69, 0x76,
	0x65, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x08, 0x52, 0x0f, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67,
	0x47, 0x70, 0x75, 0x44, 0x72, 0x69, 0x76, 0x65, 0x72, 0x12, 0x2a, 0x0a, 0x11, 0x67, 0x70, 0x75,
	0x5f, 0x64, 0x65, 0x76, 0x69, 0x63, 0x65, 0x5f, 0x70, 0x6c, 0x75, 0x67, 0x69, 0x6e, 0x18, 0x03,
	0x20, 0x01, 0x28, 0x08, 0x52, 0x0f, 0x67, 0x70, 0x75, 0x44, 0x65, 0x76, 0x69, 0x63, 0x65, 0x50,
	0x6c, 0x75, 0x67, 0x69, 0x6e, 0x12, 0x30, 0x0a, 0x14, 0x67, 0x70, 0x75, 0x5f, 0x69, 0x6e, 0x73,
	0x74, 0x61, 0x6e, 0x63, 0x65, 0x5f, 0x70, 0x72, 0x6f, 0x66, 0x69, 0x6c, 0x65, 0x18, 0x04, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x12, 0x67, 0x70, 0x75, 0x49, 0x6e, 0x73, 0x74, 0x61, 0x6e, 0x63, 0x65,
	0x50, 0x72, 0x6f, 0x66, 0x69, 0x6c, 0x65, 0x12, 0x29, 0x0a, 0x0e, 0x65, 0x6e, 0x61, 0x62, 0x6c,
	0x65, 0x5f, 0x61, 0x6d, 0x64, 0x5f, 0x67, 0x70, 0x75, 0x18, 0x05, 0x20, 0x01, 0x28, 0x08, 0x48,
	0x01, 0x52, 0x0c, 0x65, 0x6e, 0x61, 0x62, 0x6c, 0x65, 0x41, 0x6d, 0x64, 0x47, 0x70, 0x75, 0x88,
	0x01, 0x01, 0x42, 0x10, 0x0a, 0x0e, 0x5f, 0x65, 0x6e, 0x61, 0x62, 0x6c, 0x65, 0x5f, 0x6e, 0x76,
	0x69, 0x64, 0x69, 0x61, 0x42, 0x11, 0x0a, 0x0f, 0x5f, 0x65, 0x6e, 0x61, 0x62, 0x6c, 0x65, 0x5f,
	0x61, 0x6d, 0x64, 0x5f, 0x67, 0x70, 0x75, 0x42, 0x5a, 0x5a, 0x58, 0x67, 0x69, 0x74, 0x68, 0x75,
	0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x41, 0x7a, 0x75, 0x72, 0x65, 0x2f, 0x61, 0x67, 0x65, 0x6e,
	0x74, 0x62, 0x61, 0x6b, 0x65, 0x72, 0x2f, 0x61, 0x6b, 0x73, 0x2d, 0x6e, 0x6f, 0x64, 0x65, 0x2d,
	0x63, 0x6f, 0x6e, 0x74, 0x72, 0x6f, 0x6c, 0x6c, 0x65, 0x72, 0x2f, 0x70, 0x6b, 0x67, 0x2f, 0x67,
	0x65, 0x6e, 0x2f, 0x61, 0x6b, 0x73, 0x6e, 0x6f, 0x64, 0x65, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67,
	0x2f, 0x76, 0x31, 0x3b, 0x61, 0x6b, 0x73, 0x6e, 0x6f, 0x64, 0x65, 0x63, 0x6f, 0x6e, 0x66, 0x69,
	0x67, 0x76, 0x31, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_aksnodeconfig_v1_gpu_config_proto_rawDescOnce sync.Once
	file_aksnodeconfig_v1_gpu_config_proto_rawDescData = file_aksnodeconfig_v1_gpu_config_proto_rawDesc
)

func file_aksnodeconfig_v1_gpu_config_proto_rawDescGZIP() []byte {
	file_aksnodeconfig_v1_gpu_config_proto_rawDescOnce.Do(func() {
		file_aksnodeconfig_v1_gpu_config_proto_rawDescData = protoimpl.X.CompressGZIP(file_aksnodeconfig_v1_gpu_config_proto_rawDescData)
	})
	return file_aksnodeconfig_v1_gpu_config_proto_rawDescData
}

var file_aksnodeconfig_v1_gpu_config_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_aksnodeconfig_v1_gpu_config_proto_goTypes = []any{
	(*GpuConfig)(nil), // 0: aksnodeconfig.v1.GpuConfig
}
var file_aksnodeconfig_v1_gpu_config_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_aksnodeconfig_v1_gpu_config_proto_init() }
func file_aksnodeconfig_v1_gpu_config_proto_init() {
	if File_aksnodeconfig_v1_gpu_config_proto != nil {
		return
	}
	file_aksnodeconfig_v1_gpu_config_proto_msgTypes[0].OneofWrappers = []any{}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_aksnodeconfig_v1_gpu_config_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_aksnodeconfig_v1_gpu_config_proto_goTypes,
		DependencyIndexes: file_aksnodeconfig_v1_gpu_config_proto_depIdxs,
		MessageInfos:      file_aksnodeconfig_v1_gpu_config_proto_msgTypes,
	}.Build()
	File_aksnodeconfig_v1_gpu_config_proto = out.File
	file_aksnodeconfig_v1_gpu_config_proto_rawDesc = nil
	file_aksnodeconfig_v1_gpu_config_proto_goTypes = nil
	file_aksnodeconfig_v1_gpu_config_proto_depIdxs = nil
}
