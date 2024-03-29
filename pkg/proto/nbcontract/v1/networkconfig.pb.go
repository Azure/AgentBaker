// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.33.0
// 	protoc        (unknown)
// source: pkg/proto/nbcontract/v1/networkconfig.proto

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

type NetworkModeType int32

const (
	NetworkModeType_NETWORK_MODE_UNSPECIFIED NetworkModeType = 0
	NetworkModeType_NETWORK_MODE_L2BRIDGE    NetworkModeType = 1
	NetworkModeType_NETWORK_MODE_TRANSPARENT NetworkModeType = 2 //could be more. Needs to check.
)

// Enum value maps for NetworkModeType.
var (
	NetworkModeType_name = map[int32]string{
		0: "NETWORK_MODE_UNSPECIFIED",
		1: "NETWORK_MODE_L2BRIDGE",
		2: "NETWORK_MODE_TRANSPARENT",
	}
	NetworkModeType_value = map[string]int32{
		"NETWORK_MODE_UNSPECIFIED": 0,
		"NETWORK_MODE_L2BRIDGE":    1,
		"NETWORK_MODE_TRANSPARENT": 2,
	}
)

func (x NetworkModeType) Enum() *NetworkModeType {
	p := new(NetworkModeType)
	*p = x
	return p
}

func (x NetworkModeType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (NetworkModeType) Descriptor() protoreflect.EnumDescriptor {
	return file_pkg_proto_nbcontract_v1_networkconfig_proto_enumTypes[0].Descriptor()
}

func (NetworkModeType) Type() protoreflect.EnumType {
	return &file_pkg_proto_nbcontract_v1_networkconfig_proto_enumTypes[0]
}

func (x NetworkModeType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use NetworkModeType.Descriptor instead.
func (NetworkModeType) EnumDescriptor() ([]byte, []int) {
	return file_pkg_proto_nbcontract_v1_networkconfig_proto_rawDescGZIP(), []int{0}
}

type NetworkPluginType int32

const (
	NetworkPluginType_NETWORK_PLUGIN_TYPE_UNSPECIFIED NetworkPluginType = 0
	NetworkPluginType_NETWORK_PLUGIN_TYPE_NONE        NetworkPluginType = 1
	NetworkPluginType_NETWORK_PLUGIN_TYPE_AZURE       NetworkPluginType = 2
	NetworkPluginType_NETWORK_PLUGIN_TYPE_KUBENET     NetworkPluginType = 3
)

// Enum value maps for NetworkPluginType.
var (
	NetworkPluginType_name = map[int32]string{
		0: "NETWORK_PLUGIN_TYPE_UNSPECIFIED",
		1: "NETWORK_PLUGIN_TYPE_NONE",
		2: "NETWORK_PLUGIN_TYPE_AZURE",
		3: "NETWORK_PLUGIN_TYPE_KUBENET",
	}
	NetworkPluginType_value = map[string]int32{
		"NETWORK_PLUGIN_TYPE_UNSPECIFIED": 0,
		"NETWORK_PLUGIN_TYPE_NONE":        1,
		"NETWORK_PLUGIN_TYPE_AZURE":       2,
		"NETWORK_PLUGIN_TYPE_KUBENET":     3,
	}
)

func (x NetworkPluginType) Enum() *NetworkPluginType {
	p := new(NetworkPluginType)
	*p = x
	return p
}

func (x NetworkPluginType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (NetworkPluginType) Descriptor() protoreflect.EnumDescriptor {
	return file_pkg_proto_nbcontract_v1_networkconfig_proto_enumTypes[1].Descriptor()
}

func (NetworkPluginType) Type() protoreflect.EnumType {
	return &file_pkg_proto_nbcontract_v1_networkconfig_proto_enumTypes[1]
}

func (x NetworkPluginType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use NetworkPluginType.Descriptor instead.
func (NetworkPluginType) EnumDescriptor() ([]byte, []int) {
	return file_pkg_proto_nbcontract_v1_networkconfig_proto_rawDescGZIP(), []int{1}
}

type NetworkPolicyType int32

const (
	NetworkPolicyType_NETWORK_POLICY_TYPE_UNSPECIFIED NetworkPolicyType = 0
	NetworkPolicyType_NETWORK_POLICY_TYPE_NONE        NetworkPolicyType = 1
	NetworkPolicyType_NETWORK_POLICY_TYPE_AZURE       NetworkPolicyType = 2
	NetworkPolicyType_NETWORK_POLICY_TYPE_CALICO      NetworkPolicyType = 3
)

// Enum value maps for NetworkPolicyType.
var (
	NetworkPolicyType_name = map[int32]string{
		0: "NETWORK_POLICY_TYPE_UNSPECIFIED",
		1: "NETWORK_POLICY_TYPE_NONE",
		2: "NETWORK_POLICY_TYPE_AZURE",
		3: "NETWORK_POLICY_TYPE_CALICO",
	}
	NetworkPolicyType_value = map[string]int32{
		"NETWORK_POLICY_TYPE_UNSPECIFIED": 0,
		"NETWORK_POLICY_TYPE_NONE":        1,
		"NETWORK_POLICY_TYPE_AZURE":       2,
		"NETWORK_POLICY_TYPE_CALICO":      3,
	}
)

func (x NetworkPolicyType) Enum() *NetworkPolicyType {
	p := new(NetworkPolicyType)
	*p = x
	return p
}

func (x NetworkPolicyType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (NetworkPolicyType) Descriptor() protoreflect.EnumDescriptor {
	return file_pkg_proto_nbcontract_v1_networkconfig_proto_enumTypes[2].Descriptor()
}

func (NetworkPolicyType) Type() protoreflect.EnumType {
	return &file_pkg_proto_nbcontract_v1_networkconfig_proto_enumTypes[2]
}

func (x NetworkPolicyType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use NetworkPolicyType.Descriptor instead.
func (NetworkPolicyType) EnumDescriptor() ([]byte, []int) {
	return file_pkg_proto_nbcontract_v1_networkconfig_proto_rawDescGZIP(), []int{2}
}

type NetworkConfig struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	NetworkPlugin     NetworkPluginType `protobuf:"varint,1,opt,name=network_plugin,json=networkPlugin,proto3,enum=nbcontract.v1.NetworkPluginType" json:"network_plugin,omitempty"`
	NetworkPolicy     NetworkPolicyType `protobuf:"varint,2,opt,name=network_policy,json=networkPolicy,proto3,enum=nbcontract.v1.NetworkPolicyType" json:"network_policy,omitempty"`
	NetworkMode       NetworkModeType   `protobuf:"varint,3,opt,name=network_mode,json=networkMode,proto3,enum=nbcontract.v1.NetworkModeType" json:"network_mode,omitempty"`
	VnetCniPluginsUrl string            `protobuf:"bytes,4,opt,name=vnet_cni_plugins_url,json=vnetCniPluginsUrl,proto3" json:"vnet_cni_plugins_url,omitempty"`
	CniPluginsUrl     string            `protobuf:"bytes,5,opt,name=cni_plugins_url,json=cniPluginsUrl,proto3" json:"cni_plugins_url,omitempty"`
}

func (x *NetworkConfig) Reset() {
	*x = NetworkConfig{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_proto_nbcontract_v1_networkconfig_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *NetworkConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NetworkConfig) ProtoMessage() {}

func (x *NetworkConfig) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_proto_nbcontract_v1_networkconfig_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NetworkConfig.ProtoReflect.Descriptor instead.
func (*NetworkConfig) Descriptor() ([]byte, []int) {
	return file_pkg_proto_nbcontract_v1_networkconfig_proto_rawDescGZIP(), []int{0}
}

func (x *NetworkConfig) GetNetworkPlugin() NetworkPluginType {
	if x != nil {
		return x.NetworkPlugin
	}
	return NetworkPluginType_NETWORK_PLUGIN_TYPE_UNSPECIFIED
}

func (x *NetworkConfig) GetNetworkPolicy() NetworkPolicyType {
	if x != nil {
		return x.NetworkPolicy
	}
	return NetworkPolicyType_NETWORK_POLICY_TYPE_UNSPECIFIED
}

func (x *NetworkConfig) GetNetworkMode() NetworkModeType {
	if x != nil {
		return x.NetworkMode
	}
	return NetworkModeType_NETWORK_MODE_UNSPECIFIED
}

func (x *NetworkConfig) GetVnetCniPluginsUrl() string {
	if x != nil {
		return x.VnetCniPluginsUrl
	}
	return ""
}

func (x *NetworkConfig) GetCniPluginsUrl() string {
	if x != nil {
		return x.CniPluginsUrl
	}
	return ""
}

var File_pkg_proto_nbcontract_v1_networkconfig_proto protoreflect.FileDescriptor

var file_pkg_proto_nbcontract_v1_networkconfig_proto_rawDesc = []byte{
	0x0a, 0x2b, 0x70, 0x6b, 0x67, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x6e, 0x62, 0x63, 0x6f,
	0x6e, 0x74, 0x72, 0x61, 0x63, 0x74, 0x2f, 0x76, 0x31, 0x2f, 0x6e, 0x65, 0x74, 0x77, 0x6f, 0x72,
	0x6b, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0d, 0x6e,
	0x62, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63, 0x74, 0x2e, 0x76, 0x31, 0x1a, 0x2a, 0x70, 0x6b,
	0x67, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x6e, 0x62, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x61,
	0x63, 0x74, 0x2f, 0x76, 0x31, 0x2f, 0x66, 0x65, 0x61, 0x74, 0x75, 0x72, 0x65, 0x73, 0x74, 0x61,
	0x74, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xbd, 0x02, 0x0a, 0x0d, 0x4e, 0x65, 0x74,
	0x77, 0x6f, 0x72, 0x6b, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x47, 0x0a, 0x0e, 0x6e, 0x65,
	0x74, 0x77, 0x6f, 0x72, 0x6b, 0x5f, 0x70, 0x6c, 0x75, 0x67, 0x69, 0x6e, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x0e, 0x32, 0x20, 0x2e, 0x6e, 0x62, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63, 0x74, 0x2e,
	0x76, 0x31, 0x2e, 0x4e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x50, 0x6c, 0x75, 0x67, 0x69, 0x6e,
	0x54, 0x79, 0x70, 0x65, 0x52, 0x0d, 0x6e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x50, 0x6c, 0x75,
	0x67, 0x69, 0x6e, 0x12, 0x47, 0x0a, 0x0e, 0x6e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x5f, 0x70,
	0x6f, 0x6c, 0x69, 0x63, 0x79, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x20, 0x2e, 0x6e, 0x62,
	0x63, 0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63, 0x74, 0x2e, 0x76, 0x31, 0x2e, 0x4e, 0x65, 0x74, 0x77,
	0x6f, 0x72, 0x6b, 0x50, 0x6f, 0x6c, 0x69, 0x63, 0x79, 0x54, 0x79, 0x70, 0x65, 0x52, 0x0d, 0x6e,
	0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x50, 0x6f, 0x6c, 0x69, 0x63, 0x79, 0x12, 0x41, 0x0a, 0x0c,
	0x6e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x5f, 0x6d, 0x6f, 0x64, 0x65, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x0e, 0x32, 0x1e, 0x2e, 0x6e, 0x62, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63, 0x74, 0x2e,
	0x76, 0x31, 0x2e, 0x4e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x4d, 0x6f, 0x64, 0x65, 0x54, 0x79,
	0x70, 0x65, 0x52, 0x0b, 0x6e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x4d, 0x6f, 0x64, 0x65, 0x12,
	0x2f, 0x0a, 0x14, 0x76, 0x6e, 0x65, 0x74, 0x5f, 0x63, 0x6e, 0x69, 0x5f, 0x70, 0x6c, 0x75, 0x67,
	0x69, 0x6e, 0x73, 0x5f, 0x75, 0x72, 0x6c, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x11, 0x76,
	0x6e, 0x65, 0x74, 0x43, 0x6e, 0x69, 0x50, 0x6c, 0x75, 0x67, 0x69, 0x6e, 0x73, 0x55, 0x72, 0x6c,
	0x12, 0x26, 0x0a, 0x0f, 0x63, 0x6e, 0x69, 0x5f, 0x70, 0x6c, 0x75, 0x67, 0x69, 0x6e, 0x73, 0x5f,
	0x75, 0x72, 0x6c, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0d, 0x63, 0x6e, 0x69, 0x50, 0x6c,
	0x75, 0x67, 0x69, 0x6e, 0x73, 0x55, 0x72, 0x6c, 0x2a, 0x68, 0x0a, 0x0f, 0x4e, 0x65, 0x74, 0x77,
	0x6f, 0x72, 0x6b, 0x4d, 0x6f, 0x64, 0x65, 0x54, 0x79, 0x70, 0x65, 0x12, 0x1c, 0x0a, 0x18, 0x4e,
	0x45, 0x54, 0x57, 0x4f, 0x52, 0x4b, 0x5f, 0x4d, 0x4f, 0x44, 0x45, 0x5f, 0x55, 0x4e, 0x53, 0x50,
	0x45, 0x43, 0x49, 0x46, 0x49, 0x45, 0x44, 0x10, 0x00, 0x12, 0x19, 0x0a, 0x15, 0x4e, 0x45, 0x54,
	0x57, 0x4f, 0x52, 0x4b, 0x5f, 0x4d, 0x4f, 0x44, 0x45, 0x5f, 0x4c, 0x32, 0x42, 0x52, 0x49, 0x44,
	0x47, 0x45, 0x10, 0x01, 0x12, 0x1c, 0x0a, 0x18, 0x4e, 0x45, 0x54, 0x57, 0x4f, 0x52, 0x4b, 0x5f,
	0x4d, 0x4f, 0x44, 0x45, 0x5f, 0x54, 0x52, 0x41, 0x4e, 0x53, 0x50, 0x41, 0x52, 0x45, 0x4e, 0x54,
	0x10, 0x02, 0x2a, 0x96, 0x01, 0x0a, 0x11, 0x4e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x50, 0x6c,
	0x75, 0x67, 0x69, 0x6e, 0x54, 0x79, 0x70, 0x65, 0x12, 0x23, 0x0a, 0x1f, 0x4e, 0x45, 0x54, 0x57,
	0x4f, 0x52, 0x4b, 0x5f, 0x50, 0x4c, 0x55, 0x47, 0x49, 0x4e, 0x5f, 0x54, 0x59, 0x50, 0x45, 0x5f,
	0x55, 0x4e, 0x53, 0x50, 0x45, 0x43, 0x49, 0x46, 0x49, 0x45, 0x44, 0x10, 0x00, 0x12, 0x1c, 0x0a,
	0x18, 0x4e, 0x45, 0x54, 0x57, 0x4f, 0x52, 0x4b, 0x5f, 0x50, 0x4c, 0x55, 0x47, 0x49, 0x4e, 0x5f,
	0x54, 0x59, 0x50, 0x45, 0x5f, 0x4e, 0x4f, 0x4e, 0x45, 0x10, 0x01, 0x12, 0x1d, 0x0a, 0x19, 0x4e,
	0x45, 0x54, 0x57, 0x4f, 0x52, 0x4b, 0x5f, 0x50, 0x4c, 0x55, 0x47, 0x49, 0x4e, 0x5f, 0x54, 0x59,
	0x50, 0x45, 0x5f, 0x41, 0x5a, 0x55, 0x52, 0x45, 0x10, 0x02, 0x12, 0x1f, 0x0a, 0x1b, 0x4e, 0x45,
	0x54, 0x57, 0x4f, 0x52, 0x4b, 0x5f, 0x50, 0x4c, 0x55, 0x47, 0x49, 0x4e, 0x5f, 0x54, 0x59, 0x50,
	0x45, 0x5f, 0x4b, 0x55, 0x42, 0x45, 0x4e, 0x45, 0x54, 0x10, 0x03, 0x2a, 0x95, 0x01, 0x0a, 0x11,
	0x4e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x50, 0x6f, 0x6c, 0x69, 0x63, 0x79, 0x54, 0x79, 0x70,
	0x65, 0x12, 0x23, 0x0a, 0x1f, 0x4e, 0x45, 0x54, 0x57, 0x4f, 0x52, 0x4b, 0x5f, 0x50, 0x4f, 0x4c,
	0x49, 0x43, 0x59, 0x5f, 0x54, 0x59, 0x50, 0x45, 0x5f, 0x55, 0x4e, 0x53, 0x50, 0x45, 0x43, 0x49,
	0x46, 0x49, 0x45, 0x44, 0x10, 0x00, 0x12, 0x1c, 0x0a, 0x18, 0x4e, 0x45, 0x54, 0x57, 0x4f, 0x52,
	0x4b, 0x5f, 0x50, 0x4f, 0x4c, 0x49, 0x43, 0x59, 0x5f, 0x54, 0x59, 0x50, 0x45, 0x5f, 0x4e, 0x4f,
	0x4e, 0x45, 0x10, 0x01, 0x12, 0x1d, 0x0a, 0x19, 0x4e, 0x45, 0x54, 0x57, 0x4f, 0x52, 0x4b, 0x5f,
	0x50, 0x4f, 0x4c, 0x49, 0x43, 0x59, 0x5f, 0x54, 0x59, 0x50, 0x45, 0x5f, 0x41, 0x5a, 0x55, 0x52,
	0x45, 0x10, 0x02, 0x12, 0x1e, 0x0a, 0x1a, 0x4e, 0x45, 0x54, 0x57, 0x4f, 0x52, 0x4b, 0x5f, 0x50,
	0x4f, 0x4c, 0x49, 0x43, 0x59, 0x5f, 0x54, 0x59, 0x50, 0x45, 0x5f, 0x43, 0x41, 0x4c, 0x49, 0x43,
	0x4f, 0x10, 0x03, 0x42, 0xbe, 0x01, 0x0a, 0x11, 0x63, 0x6f, 0x6d, 0x2e, 0x6e, 0x62, 0x63, 0x6f,
	0x6e, 0x74, 0x72, 0x61, 0x63, 0x74, 0x2e, 0x76, 0x31, 0x42, 0x12, 0x4e, 0x65, 0x74, 0x77, 0x6f,
	0x72, 0x6b, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a,
	0x40, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x41, 0x7a, 0x75, 0x72,
	0x65, 0x2f, 0x41, 0x67, 0x65, 0x6e, 0x74, 0x42, 0x61, 0x6b, 0x65, 0x72, 0x2f, 0x70, 0x6b, 0x67,
	0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x6e, 0x62, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63,
	0x74, 0x2f, 0x76, 0x31, 0x3b, 0x6e, 0x62, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63, 0x74, 0x76,
	0x31, 0xa2, 0x02, 0x03, 0x4e, 0x58, 0x58, 0xaa, 0x02, 0x0d, 0x4e, 0x62, 0x63, 0x6f, 0x6e, 0x74,
	0x72, 0x61, 0x63, 0x74, 0x2e, 0x56, 0x31, 0xca, 0x02, 0x0d, 0x4e, 0x62, 0x63, 0x6f, 0x6e, 0x74,
	0x72, 0x61, 0x63, 0x74, 0x5c, 0x56, 0x31, 0xe2, 0x02, 0x19, 0x4e, 0x62, 0x63, 0x6f, 0x6e, 0x74,
	0x72, 0x61, 0x63, 0x74, 0x5c, 0x56, 0x31, 0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65, 0x74, 0x61, 0x64,
	0x61, 0x74, 0x61, 0xea, 0x02, 0x0e, 0x4e, 0x62, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x61, 0x63, 0x74,
	0x3a, 0x3a, 0x56, 0x31, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_pkg_proto_nbcontract_v1_networkconfig_proto_rawDescOnce sync.Once
	file_pkg_proto_nbcontract_v1_networkconfig_proto_rawDescData = file_pkg_proto_nbcontract_v1_networkconfig_proto_rawDesc
)

func file_pkg_proto_nbcontract_v1_networkconfig_proto_rawDescGZIP() []byte {
	file_pkg_proto_nbcontract_v1_networkconfig_proto_rawDescOnce.Do(func() {
		file_pkg_proto_nbcontract_v1_networkconfig_proto_rawDescData = protoimpl.X.CompressGZIP(file_pkg_proto_nbcontract_v1_networkconfig_proto_rawDescData)
	})
	return file_pkg_proto_nbcontract_v1_networkconfig_proto_rawDescData
}

var file_pkg_proto_nbcontract_v1_networkconfig_proto_enumTypes = make([]protoimpl.EnumInfo, 3)
var file_pkg_proto_nbcontract_v1_networkconfig_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_pkg_proto_nbcontract_v1_networkconfig_proto_goTypes = []interface{}{
	(NetworkModeType)(0),   // 0: nbcontract.v1.NetworkModeType
	(NetworkPluginType)(0), // 1: nbcontract.v1.NetworkPluginType
	(NetworkPolicyType)(0), // 2: nbcontract.v1.NetworkPolicyType
	(*NetworkConfig)(nil),  // 3: nbcontract.v1.NetworkConfig
}
var file_pkg_proto_nbcontract_v1_networkconfig_proto_depIdxs = []int32{
	1, // 0: nbcontract.v1.NetworkConfig.network_plugin:type_name -> nbcontract.v1.NetworkPluginType
	2, // 1: nbcontract.v1.NetworkConfig.network_policy:type_name -> nbcontract.v1.NetworkPolicyType
	0, // 2: nbcontract.v1.NetworkConfig.network_mode:type_name -> nbcontract.v1.NetworkModeType
	3, // [3:3] is the sub-list for method output_type
	3, // [3:3] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_pkg_proto_nbcontract_v1_networkconfig_proto_init() }
func file_pkg_proto_nbcontract_v1_networkconfig_proto_init() {
	if File_pkg_proto_nbcontract_v1_networkconfig_proto != nil {
		return
	}
	file_pkg_proto_nbcontract_v1_featurestate_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_pkg_proto_nbcontract_v1_networkconfig_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*NetworkConfig); i {
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
			RawDescriptor: file_pkg_proto_nbcontract_v1_networkconfig_proto_rawDesc,
			NumEnums:      3,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_pkg_proto_nbcontract_v1_networkconfig_proto_goTypes,
		DependencyIndexes: file_pkg_proto_nbcontract_v1_networkconfig_proto_depIdxs,
		EnumInfos:         file_pkg_proto_nbcontract_v1_networkconfig_proto_enumTypes,
		MessageInfos:      file_pkg_proto_nbcontract_v1_networkconfig_proto_msgTypes,
	}.Build()
	File_pkg_proto_nbcontract_v1_networkconfig_proto = out.File
	file_pkg_proto_nbcontract_v1_networkconfig_proto_rawDesc = nil
	file_pkg_proto_nbcontract_v1_networkconfig_proto_goTypes = nil
	file_pkg_proto_nbcontract_v1_networkconfig_proto_depIdxs = nil
}
