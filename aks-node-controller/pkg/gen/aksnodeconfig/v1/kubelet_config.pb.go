// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.35.2
// 	protoc        v5.28.3
// source: aksnodeconfig/v1/kubelet_config.proto

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

type KubeletDisk int32

const (
	KubeletDisk_KUBELET_DISK_UNSPECIFIED KubeletDisk = 0
	KubeletDisk_KUBELET_DISK_OS_DISK     KubeletDisk = 1
	KubeletDisk_KUBELET_DISK_TEMP_DISK   KubeletDisk = 2
)

// Enum value maps for KubeletDisk.
var (
	KubeletDisk_name = map[int32]string{
		0: "KUBELET_DISK_UNSPECIFIED",
		1: "KUBELET_DISK_OS_DISK",
		2: "KUBELET_DISK_TEMP_DISK",
	}
	KubeletDisk_value = map[string]int32{
		"KUBELET_DISK_UNSPECIFIED": 0,
		"KUBELET_DISK_OS_DISK":     1,
		"KUBELET_DISK_TEMP_DISK":   2,
	}
)

func (x KubeletDisk) Enum() *KubeletDisk {
	p := new(KubeletDisk)
	*p = x
	return p
}

func (x KubeletDisk) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (KubeletDisk) Descriptor() protoreflect.EnumDescriptor {
	return file_aksnodeconfig_v1_kubelet_config_proto_enumTypes[0].Descriptor()
}

func (KubeletDisk) Type() protoreflect.EnumType {
	return &file_aksnodeconfig_v1_kubelet_config_proto_enumTypes[0]
}

func (x KubeletDisk) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use KubeletDisk.Descriptor instead.
func (KubeletDisk) EnumDescriptor() ([]byte, []int) {
	return file_aksnodeconfig_v1_kubelet_config_proto_rawDescGZIP(), []int{0}
}

type KubeletConfig struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// A list of taints to apply to the node.
	// These are reserved for now. We are using KubeletFlags with flag "--register-with-taints" to register node with taints.
	Taints []*Taint `protobuf:"bytes,1,rep,name=taints,proto3" json:"taints,omitempty"`
	// A map of kubelet flags to their values.
	KubeletFlags map[string]string `protobuf:"bytes,2,rep,name=kubelet_flags,json=kubeletFlags,proto3" json:"kubelet_flags,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	// A map of node labels to their values.
	KubeletNodeLabels map[string]string `protobuf:"bytes,3,rep,name=kubelet_node_labels,json=kubeletNodeLabels,proto3" json:"kubelet_node_labels,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	// A list of taints to apply to the node at startup.
	// These are reserved for now. We are using KubeletFlags with flag "--register-with-taints" to register node with taints.
	StartupTaints []*Taint `protobuf:"bytes,4,rep,name=startup_taints,json=startupTaints,proto3" json:"startup_taints,omitempty"`
	// The type of disk to use for the kubelet.
	KubeletDiskType KubeletDisk `protobuf:"varint,5,opt,name=kubelet_disk_type,json=kubeletDiskType,proto3,enum=aksnodeconfig.v1.KubeletDisk" json:"kubelet_disk_type,omitempty"`
	// Whether kubelet config should be enabled.
	EnableKubeletConfigFile bool `protobuf:"varint,6,opt,name=enable_kubelet_config_file,json=enableKubeletConfigFile,proto3" json:"enable_kubelet_config_file,omitempty"`
	// Base64 encoded content of the kubelet config file.
	KubeletConfigFileContent string `protobuf:"bytes,7,opt,name=kubelet_config_file_content,json=kubeletConfigFileContent,proto3" json:"kubelet_config_file_content,omitempty"`
	// Kubelet client private key
	KubeletClientKey string `protobuf:"bytes,8,opt,name=kubelet_client_key,json=kubeletClientKey,proto3" json:"kubelet_client_key,omitempty"`
	// The content of the kubelet client certificate file.
	KubeletClientCertContent string `protobuf:"bytes,9,opt,name=kubelet_client_cert_content,json=kubeletClientCertContent,proto3" json:"kubelet_client_cert_content,omitempty"`
	// The path used to mount docker images, emptyDir volumes, and kubelet data.
	ContainerDataDir string `protobuf:"bytes,10,opt,name=container_data_dir,json=containerDataDir,proto3" json:"container_data_dir,omitempty"`
}

func (x *KubeletConfig) Reset() {
	*x = KubeletConfig{}
	mi := &file_aksnodeconfig_v1_kubelet_config_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *KubeletConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*KubeletConfig) ProtoMessage() {}

func (x *KubeletConfig) ProtoReflect() protoreflect.Message {
	mi := &file_aksnodeconfig_v1_kubelet_config_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use KubeletConfig.ProtoReflect.Descriptor instead.
func (*KubeletConfig) Descriptor() ([]byte, []int) {
	return file_aksnodeconfig_v1_kubelet_config_proto_rawDescGZIP(), []int{0}
}

func (x *KubeletConfig) GetTaints() []*Taint {
	if x != nil {
		return x.Taints
	}
	return nil
}

func (x *KubeletConfig) GetKubeletFlags() map[string]string {
	if x != nil {
		return x.KubeletFlags
	}
	return nil
}

func (x *KubeletConfig) GetKubeletNodeLabels() map[string]string {
	if x != nil {
		return x.KubeletNodeLabels
	}
	return nil
}

func (x *KubeletConfig) GetStartupTaints() []*Taint {
	if x != nil {
		return x.StartupTaints
	}
	return nil
}

func (x *KubeletConfig) GetKubeletDiskType() KubeletDisk {
	if x != nil {
		return x.KubeletDiskType
	}
	return KubeletDisk_KUBELET_DISK_UNSPECIFIED
}

func (x *KubeletConfig) GetEnableKubeletConfigFile() bool {
	if x != nil {
		return x.EnableKubeletConfigFile
	}
	return false
}

func (x *KubeletConfig) GetKubeletConfigFileContent() string {
	if x != nil {
		return x.KubeletConfigFileContent
	}
	return ""
}

func (x *KubeletConfig) GetKubeletClientKey() string {
	if x != nil {
		return x.KubeletClientKey
	}
	return ""
}

func (x *KubeletConfig) GetKubeletClientCertContent() string {
	if x != nil {
		return x.KubeletClientCertContent
	}
	return ""
}

func (x *KubeletConfig) GetContainerDataDir() string {
	if x != nil {
		return x.ContainerDataDir
	}
	return ""
}

type Taint struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Key    string `protobuf:"bytes,1,opt,name=key,proto3" json:"key,omitempty"`
	Effect string `protobuf:"bytes,2,opt,name=effect,proto3" json:"effect,omitempty"`
	Value  string `protobuf:"bytes,3,opt,name=value,proto3" json:"value,omitempty"`
}

func (x *Taint) Reset() {
	*x = Taint{}
	mi := &file_aksnodeconfig_v1_kubelet_config_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Taint) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Taint) ProtoMessage() {}

func (x *Taint) ProtoReflect() protoreflect.Message {
	mi := &file_aksnodeconfig_v1_kubelet_config_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Taint.ProtoReflect.Descriptor instead.
func (*Taint) Descriptor() ([]byte, []int) {
	return file_aksnodeconfig_v1_kubelet_config_proto_rawDescGZIP(), []int{1}
}

func (x *Taint) GetKey() string {
	if x != nil {
		return x.Key
	}
	return ""
}

func (x *Taint) GetEffect() string {
	if x != nil {
		return x.Effect
	}
	return ""
}

func (x *Taint) GetValue() string {
	if x != nil {
		return x.Value
	}
	return ""
}

var File_aksnodeconfig_v1_kubelet_config_proto protoreflect.FileDescriptor

var file_aksnodeconfig_v1_kubelet_config_proto_rawDesc = []byte{
	0x0a, 0x25, 0x61, 0x6b, 0x73, 0x6e, 0x6f, 0x64, 0x65, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2f,
	0x76, 0x31, 0x2f, 0x6b, 0x75, 0x62, 0x65, 0x6c, 0x65, 0x74, 0x5f, 0x63, 0x6f, 0x6e, 0x66, 0x69,
	0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x10, 0x61, 0x6b, 0x73, 0x6e, 0x6f, 0x64, 0x65,
	0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x76, 0x31, 0x22, 0xa9, 0x06, 0x0a, 0x0d, 0x4b, 0x75,
	0x62, 0x65, 0x6c, 0x65, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x2f, 0x0a, 0x06, 0x74,
	0x61, 0x69, 0x6e, 0x74, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x17, 0x2e, 0x61, 0x6b,
	0x73, 0x6e, 0x6f, 0x64, 0x65, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x76, 0x31, 0x2e, 0x54,
	0x61, 0x69, 0x6e, 0x74, 0x52, 0x06, 0x74, 0x61, 0x69, 0x6e, 0x74, 0x73, 0x12, 0x56, 0x0a, 0x0d,
	0x6b, 0x75, 0x62, 0x65, 0x6c, 0x65, 0x74, 0x5f, 0x66, 0x6c, 0x61, 0x67, 0x73, 0x18, 0x02, 0x20,
	0x03, 0x28, 0x0b, 0x32, 0x31, 0x2e, 0x61, 0x6b, 0x73, 0x6e, 0x6f, 0x64, 0x65, 0x63, 0x6f, 0x6e,
	0x66, 0x69, 0x67, 0x2e, 0x76, 0x31, 0x2e, 0x4b, 0x75, 0x62, 0x65, 0x6c, 0x65, 0x74, 0x43, 0x6f,
	0x6e, 0x66, 0x69, 0x67, 0x2e, 0x4b, 0x75, 0x62, 0x65, 0x6c, 0x65, 0x74, 0x46, 0x6c, 0x61, 0x67,
	0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x0c, 0x6b, 0x75, 0x62, 0x65, 0x6c, 0x65, 0x74, 0x46,
	0x6c, 0x61, 0x67, 0x73, 0x12, 0x66, 0x0a, 0x13, 0x6b, 0x75, 0x62, 0x65, 0x6c, 0x65, 0x74, 0x5f,
	0x6e, 0x6f, 0x64, 0x65, 0x5f, 0x6c, 0x61, 0x62, 0x65, 0x6c, 0x73, 0x18, 0x03, 0x20, 0x03, 0x28,
	0x0b, 0x32, 0x36, 0x2e, 0x61, 0x6b, 0x73, 0x6e, 0x6f, 0x64, 0x65, 0x63, 0x6f, 0x6e, 0x66, 0x69,
	0x67, 0x2e, 0x76, 0x31, 0x2e, 0x4b, 0x75, 0x62, 0x65, 0x6c, 0x65, 0x74, 0x43, 0x6f, 0x6e, 0x66,
	0x69, 0x67, 0x2e, 0x4b, 0x75, 0x62, 0x65, 0x6c, 0x65, 0x74, 0x4e, 0x6f, 0x64, 0x65, 0x4c, 0x61,
	0x62, 0x65, 0x6c, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x11, 0x6b, 0x75, 0x62, 0x65, 0x6c,
	0x65, 0x74, 0x4e, 0x6f, 0x64, 0x65, 0x4c, 0x61, 0x62, 0x65, 0x6c, 0x73, 0x12, 0x3e, 0x0a, 0x0e,
	0x73, 0x74, 0x61, 0x72, 0x74, 0x75, 0x70, 0x5f, 0x74, 0x61, 0x69, 0x6e, 0x74, 0x73, 0x18, 0x04,
	0x20, 0x03, 0x28, 0x0b, 0x32, 0x17, 0x2e, 0x61, 0x6b, 0x73, 0x6e, 0x6f, 0x64, 0x65, 0x63, 0x6f,
	0x6e, 0x66, 0x69, 0x67, 0x2e, 0x76, 0x31, 0x2e, 0x54, 0x61, 0x69, 0x6e, 0x74, 0x52, 0x0d, 0x73,
	0x74, 0x61, 0x72, 0x74, 0x75, 0x70, 0x54, 0x61, 0x69, 0x6e, 0x74, 0x73, 0x12, 0x49, 0x0a, 0x11,
	0x6b, 0x75, 0x62, 0x65, 0x6c, 0x65, 0x74, 0x5f, 0x64, 0x69, 0x73, 0x6b, 0x5f, 0x74, 0x79, 0x70,
	0x65, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x1d, 0x2e, 0x61, 0x6b, 0x73, 0x6e, 0x6f, 0x64,
	0x65, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x76, 0x31, 0x2e, 0x4b, 0x75, 0x62, 0x65, 0x6c,
	0x65, 0x74, 0x44, 0x69, 0x73, 0x6b, 0x52, 0x0f, 0x6b, 0x75, 0x62, 0x65, 0x6c, 0x65, 0x74, 0x44,
	0x69, 0x73, 0x6b, 0x54, 0x79, 0x70, 0x65, 0x12, 0x3b, 0x0a, 0x1a, 0x65, 0x6e, 0x61, 0x62, 0x6c,
	0x65, 0x5f, 0x6b, 0x75, 0x62, 0x65, 0x6c, 0x65, 0x74, 0x5f, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67,
	0x5f, 0x66, 0x69, 0x6c, 0x65, 0x18, 0x06, 0x20, 0x01, 0x28, 0x08, 0x52, 0x17, 0x65, 0x6e, 0x61,
	0x62, 0x6c, 0x65, 0x4b, 0x75, 0x62, 0x65, 0x6c, 0x65, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67,
	0x46, 0x69, 0x6c, 0x65, 0x12, 0x3d, 0x0a, 0x1b, 0x6b, 0x75, 0x62, 0x65, 0x6c, 0x65, 0x74, 0x5f,
	0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x5f, 0x66, 0x69, 0x6c, 0x65, 0x5f, 0x63, 0x6f, 0x6e, 0x74,
	0x65, 0x6e, 0x74, 0x18, 0x07, 0x20, 0x01, 0x28, 0x09, 0x52, 0x18, 0x6b, 0x75, 0x62, 0x65, 0x6c,
	0x65, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x46, 0x69, 0x6c, 0x65, 0x43, 0x6f, 0x6e, 0x74,
	0x65, 0x6e, 0x74, 0x12, 0x2c, 0x0a, 0x12, 0x6b, 0x75, 0x62, 0x65, 0x6c, 0x65, 0x74, 0x5f, 0x63,
	0x6c, 0x69, 0x65, 0x6e, 0x74, 0x5f, 0x6b, 0x65, 0x79, 0x18, 0x08, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x10, 0x6b, 0x75, 0x62, 0x65, 0x6c, 0x65, 0x74, 0x43, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x4b, 0x65,
	0x79, 0x12, 0x3d, 0x0a, 0x1b, 0x6b, 0x75, 0x62, 0x65, 0x6c, 0x65, 0x74, 0x5f, 0x63, 0x6c, 0x69,
	0x65, 0x6e, 0x74, 0x5f, 0x63, 0x65, 0x72, 0x74, 0x5f, 0x63, 0x6f, 0x6e, 0x74, 0x65, 0x6e, 0x74,
	0x18, 0x09, 0x20, 0x01, 0x28, 0x09, 0x52, 0x18, 0x6b, 0x75, 0x62, 0x65, 0x6c, 0x65, 0x74, 0x43,
	0x6c, 0x69, 0x65, 0x6e, 0x74, 0x43, 0x65, 0x72, 0x74, 0x43, 0x6f, 0x6e, 0x74, 0x65, 0x6e, 0x74,
	0x12, 0x2c, 0x0a, 0x12, 0x63, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x5f, 0x64, 0x61,
	0x74, 0x61, 0x5f, 0x64, 0x69, 0x72, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x09, 0x52, 0x10, 0x63, 0x6f,
	0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x44, 0x61, 0x74, 0x61, 0x44, 0x69, 0x72, 0x1a, 0x3f,
	0x0a, 0x11, 0x4b, 0x75, 0x62, 0x65, 0x6c, 0x65, 0x74, 0x46, 0x6c, 0x61, 0x67, 0x73, 0x45, 0x6e,
	0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x1a,
	0x44, 0x0a, 0x16, 0x4b, 0x75, 0x62, 0x65, 0x6c, 0x65, 0x74, 0x4e, 0x6f, 0x64, 0x65, 0x4c, 0x61,
	0x62, 0x65, 0x6c, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x76,
	0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75,
	0x65, 0x3a, 0x02, 0x38, 0x01, 0x22, 0x47, 0x0a, 0x05, 0x54, 0x61, 0x69, 0x6e, 0x74, 0x12, 0x10,
	0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79,
	0x12, 0x16, 0x0a, 0x06, 0x65, 0x66, 0x66, 0x65, 0x63, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x06, 0x65, 0x66, 0x66, 0x65, 0x63, 0x74, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75,
	0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x2a, 0x61,
	0x0a, 0x0b, 0x4b, 0x75, 0x62, 0x65, 0x6c, 0x65, 0x74, 0x44, 0x69, 0x73, 0x6b, 0x12, 0x1c, 0x0a,
	0x18, 0x4b, 0x55, 0x42, 0x45, 0x4c, 0x45, 0x54, 0x5f, 0x44, 0x49, 0x53, 0x4b, 0x5f, 0x55, 0x4e,
	0x53, 0x50, 0x45, 0x43, 0x49, 0x46, 0x49, 0x45, 0x44, 0x10, 0x00, 0x12, 0x18, 0x0a, 0x14, 0x4b,
	0x55, 0x42, 0x45, 0x4c, 0x45, 0x54, 0x5f, 0x44, 0x49, 0x53, 0x4b, 0x5f, 0x4f, 0x53, 0x5f, 0x44,
	0x49, 0x53, 0x4b, 0x10, 0x01, 0x12, 0x1a, 0x0a, 0x16, 0x4b, 0x55, 0x42, 0x45, 0x4c, 0x45, 0x54,
	0x5f, 0x44, 0x49, 0x53, 0x4b, 0x5f, 0x54, 0x45, 0x4d, 0x50, 0x5f, 0x44, 0x49, 0x53, 0x4b, 0x10,
	0x02, 0x42, 0x5a, 0x5a, 0x58, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f,
	0x41, 0x7a, 0x75, 0x72, 0x65, 0x2f, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x62, 0x61, 0x6b, 0x65, 0x72,
	0x2f, 0x61, 0x6b, 0x73, 0x2d, 0x6e, 0x6f, 0x64, 0x65, 0x2d, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x6f,
	0x6c, 0x6c, 0x65, 0x72, 0x2f, 0x70, 0x6b, 0x67, 0x2f, 0x67, 0x65, 0x6e, 0x2f, 0x61, 0x6b, 0x73,
	0x6e, 0x6f, 0x64, 0x65, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2f, 0x76, 0x31, 0x3b, 0x61, 0x6b,
	0x73, 0x6e, 0x6f, 0x64, 0x65, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x76, 0x31, 0x62, 0x06, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_aksnodeconfig_v1_kubelet_config_proto_rawDescOnce sync.Once
	file_aksnodeconfig_v1_kubelet_config_proto_rawDescData = file_aksnodeconfig_v1_kubelet_config_proto_rawDesc
)

func file_aksnodeconfig_v1_kubelet_config_proto_rawDescGZIP() []byte {
	file_aksnodeconfig_v1_kubelet_config_proto_rawDescOnce.Do(func() {
		file_aksnodeconfig_v1_kubelet_config_proto_rawDescData = protoimpl.X.CompressGZIP(file_aksnodeconfig_v1_kubelet_config_proto_rawDescData)
	})
	return file_aksnodeconfig_v1_kubelet_config_proto_rawDescData
}

var file_aksnodeconfig_v1_kubelet_config_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_aksnodeconfig_v1_kubelet_config_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_aksnodeconfig_v1_kubelet_config_proto_goTypes = []any{
	(KubeletDisk)(0),      // 0: aksnodeconfig.v1.KubeletDisk
	(*KubeletConfig)(nil), // 1: aksnodeconfig.v1.KubeletConfig
	(*Taint)(nil),         // 2: aksnodeconfig.v1.Taint
	nil,                   // 3: aksnodeconfig.v1.KubeletConfig.KubeletFlagsEntry
	nil,                   // 4: aksnodeconfig.v1.KubeletConfig.KubeletNodeLabelsEntry
}
var file_aksnodeconfig_v1_kubelet_config_proto_depIdxs = []int32{
	2, // 0: aksnodeconfig.v1.KubeletConfig.taints:type_name -> aksnodeconfig.v1.Taint
	3, // 1: aksnodeconfig.v1.KubeletConfig.kubelet_flags:type_name -> aksnodeconfig.v1.KubeletConfig.KubeletFlagsEntry
	4, // 2: aksnodeconfig.v1.KubeletConfig.kubelet_node_labels:type_name -> aksnodeconfig.v1.KubeletConfig.KubeletNodeLabelsEntry
	2, // 3: aksnodeconfig.v1.KubeletConfig.startup_taints:type_name -> aksnodeconfig.v1.Taint
	0, // 4: aksnodeconfig.v1.KubeletConfig.kubelet_disk_type:type_name -> aksnodeconfig.v1.KubeletDisk
	5, // [5:5] is the sub-list for method output_type
	5, // [5:5] is the sub-list for method input_type
	5, // [5:5] is the sub-list for extension type_name
	5, // [5:5] is the sub-list for extension extendee
	0, // [0:5] is the sub-list for field type_name
}

func init() { file_aksnodeconfig_v1_kubelet_config_proto_init() }
func file_aksnodeconfig_v1_kubelet_config_proto_init() {
	if File_aksnodeconfig_v1_kubelet_config_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_aksnodeconfig_v1_kubelet_config_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_aksnodeconfig_v1_kubelet_config_proto_goTypes,
		DependencyIndexes: file_aksnodeconfig_v1_kubelet_config_proto_depIdxs,
		EnumInfos:         file_aksnodeconfig_v1_kubelet_config_proto_enumTypes,
		MessageInfos:      file_aksnodeconfig_v1_kubelet_config_proto_msgTypes,
	}.Build()
	File_aksnodeconfig_v1_kubelet_config_proto = out.File
	file_aksnodeconfig_v1_kubelet_config_proto_rawDesc = nil
	file_aksnodeconfig_v1_kubelet_config_proto_goTypes = nil
	file_aksnodeconfig_v1_kubelet_config_proto_depIdxs = nil
}
