// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.35.2
// 	protoc        (unknown)
// source: aksnodeconfig/v1/customlinuxosconfig.proto

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

// Custom Linux Node OS Config
type CustomLinuxOSConfig struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Sysctl settings for Linux agent nodes
	SysctlConfig *SysctlConfig `protobuf:"bytes,1,opt,name=sysctl_config,json=sysctlConfig,proto3" json:"sysctl_config,omitempty"`
	// Ulimit settings for Linux agent nodes
	UlimitConfig *UlimitConfig `protobuf:"bytes,2,opt,name=ulimit_config,json=ulimitConfig,proto3" json:"ulimit_config,omitempty"`
	// Enable or disable swap configuration
	EnableSwapConfig bool `protobuf:"varint,3,opt,name=enable_swap_config,json=enableSwapConfig,proto3" json:"enable_swap_config,omitempty"`
	// The size in MB of a swap file that will be created on each node
	SwapFileSize int32 `protobuf:"varint,4,opt,name=swap_file_size,json=swapFileSize,proto3" json:"swap_file_size,omitempty"`
	// Valid values are "always", "defer", "defer+madvise", "madvise" and "never"
	// If it's unset or set to empty string, it will use the default value in the VHD "always"
	TransparentHugepageSupport string `protobuf:"bytes,5,opt,name=transparent_hugepage_support,json=transparentHugepageSupport,proto3" json:"transparent_hugepage_support,omitempty"`
	// Valid values are "always", "madvise" and "never"
	// If it's unset or set to empty string, it will use the default value in the VHD "madvise"
	TransparentDefrag string `protobuf:"bytes,6,opt,name=transparent_defrag,json=transparentDefrag,proto3" json:"transparent_defrag,omitempty"`
}

func (x *CustomLinuxOSConfig) Reset() {
	*x = CustomLinuxOSConfig{}
	mi := &file_aksnodeconfig_v1_customlinuxosconfig_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CustomLinuxOSConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CustomLinuxOSConfig) ProtoMessage() {}

func (x *CustomLinuxOSConfig) ProtoReflect() protoreflect.Message {
	mi := &file_aksnodeconfig_v1_customlinuxosconfig_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CustomLinuxOSConfig.ProtoReflect.Descriptor instead.
func (*CustomLinuxOSConfig) Descriptor() ([]byte, []int) {
	return file_aksnodeconfig_v1_customlinuxosconfig_proto_rawDescGZIP(), []int{0}
}

func (x *CustomLinuxOSConfig) GetSysctlConfig() *SysctlConfig {
	if x != nil {
		return x.SysctlConfig
	}
	return nil
}

func (x *CustomLinuxOSConfig) GetUlimitConfig() *UlimitConfig {
	if x != nil {
		return x.UlimitConfig
	}
	return nil
}

func (x *CustomLinuxOSConfig) GetEnableSwapConfig() bool {
	if x != nil {
		return x.EnableSwapConfig
	}
	return false
}

func (x *CustomLinuxOSConfig) GetSwapFileSize() int32 {
	if x != nil {
		return x.SwapFileSize
	}
	return 0
}

func (x *CustomLinuxOSConfig) GetTransparentHugepageSupport() string {
	if x != nil {
		return x.TransparentHugepageSupport
	}
	return ""
}

func (x *CustomLinuxOSConfig) GetTransparentDefrag() string {
	if x != nil {
		return x.TransparentDefrag
	}
	return ""
}

type SysctlConfig struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// using optional here to allow detecting if the field is set or not (explicit presence in proto3)
	NetCoreSomaxconn               *int32  `protobuf:"varint,1,opt,name=net_core_somaxconn,json=netCoreSomaxconn,proto3,oneof" json:"net_core_somaxconn,omitempty"`
	NetCoreNetdevMaxBacklog        *int32  `protobuf:"varint,2,opt,name=net_core_netdev_max_backlog,json=netCoreNetdevMaxBacklog,proto3,oneof" json:"net_core_netdev_max_backlog,omitempty"`
	NetCoreRmemDefault             *int32  `protobuf:"varint,3,opt,name=net_core_rmem_default,json=netCoreRmemDefault,proto3,oneof" json:"net_core_rmem_default,omitempty"`
	NetCoreRmemMax                 *int32  `protobuf:"varint,4,opt,name=net_core_rmem_max,json=netCoreRmemMax,proto3,oneof" json:"net_core_rmem_max,omitempty"`
	NetCoreWmemDefault             *int32  `protobuf:"varint,5,opt,name=net_core_wmem_default,json=netCoreWmemDefault,proto3,oneof" json:"net_core_wmem_default,omitempty"`
	NetCoreWmemMax                 *int32  `protobuf:"varint,6,opt,name=net_core_wmem_max,json=netCoreWmemMax,proto3,oneof" json:"net_core_wmem_max,omitempty"`
	NetCoreOptmemMax               *int32  `protobuf:"varint,7,opt,name=net_core_optmem_max,json=netCoreOptmemMax,proto3,oneof" json:"net_core_optmem_max,omitempty"`
	NetIpv4TcpMaxSynBacklog        *int32  `protobuf:"varint,8,opt,name=net_ipv4_tcp_max_syn_backlog,json=netIpv4TcpMaxSynBacklog,proto3,oneof" json:"net_ipv4_tcp_max_syn_backlog,omitempty"`
	NetIpv4TcpMaxTwBuckets         *int32  `protobuf:"varint,9,opt,name=net_ipv4_tcp_max_tw_buckets,json=netIpv4TcpMaxTwBuckets,proto3,oneof" json:"net_ipv4_tcp_max_tw_buckets,omitempty"`
	NetIpv4TcpFinTimeout           *int32  `protobuf:"varint,10,opt,name=net_ipv4_tcp_fin_timeout,json=netIpv4TcpFinTimeout,proto3,oneof" json:"net_ipv4_tcp_fin_timeout,omitempty"`
	NetIpv4TcpKeepaliveTime        *int32  `protobuf:"varint,11,opt,name=net_ipv4_tcp_keepalive_time,json=netIpv4TcpKeepaliveTime,proto3,oneof" json:"net_ipv4_tcp_keepalive_time,omitempty"`
	NetIpv4TcpKeepaliveProbes      *int32  `protobuf:"varint,12,opt,name=net_ipv4_tcp_keepalive_probes,json=netIpv4TcpKeepaliveProbes,proto3,oneof" json:"net_ipv4_tcp_keepalive_probes,omitempty"`
	NetIpv4TcpkeepaliveIntvl       *int32  `protobuf:"varint,13,opt,name=net_ipv4_tcpkeepalive_intvl,json=netIpv4TcpkeepaliveIntvl,proto3,oneof" json:"net_ipv4_tcpkeepalive_intvl,omitempty"`
	NetIpv4TcpTwReuse              *bool   `protobuf:"varint,14,opt,name=net_ipv4_tcp_tw_reuse,json=netIpv4TcpTwReuse,proto3,oneof" json:"net_ipv4_tcp_tw_reuse,omitempty"`
	NetIpv4IpLocalPortRange        *string `protobuf:"bytes,15,opt,name=net_ipv4_ip_local_port_range,json=netIpv4IpLocalPortRange,proto3,oneof" json:"net_ipv4_ip_local_port_range,omitempty"`
	NetIpv4NeighDefaultGcThresh1   *int32  `protobuf:"varint,16,opt,name=net_ipv4_neigh_default_gc_thresh1,json=netIpv4NeighDefaultGcThresh1,proto3,oneof" json:"net_ipv4_neigh_default_gc_thresh1,omitempty"`
	NetIpv4NeighDefaultGcThresh2   *int32  `protobuf:"varint,17,opt,name=net_ipv4_neigh_default_gc_thresh2,json=netIpv4NeighDefaultGcThresh2,proto3,oneof" json:"net_ipv4_neigh_default_gc_thresh2,omitempty"`
	NetIpv4NeighDefaultGcThresh3   *int32  `protobuf:"varint,18,opt,name=net_ipv4_neigh_default_gc_thresh3,json=netIpv4NeighDefaultGcThresh3,proto3,oneof" json:"net_ipv4_neigh_default_gc_thresh3,omitempty"`
	NetNetfilterNfConntrackMax     *int32  `protobuf:"varint,19,opt,name=net_netfilter_nf_conntrack_max,json=netNetfilterNfConntrackMax,proto3,oneof" json:"net_netfilter_nf_conntrack_max,omitempty"`
	NetNetfilterNfConntrackBuckets *int32  `protobuf:"varint,20,opt,name=net_netfilter_nf_conntrack_buckets,json=netNetfilterNfConntrackBuckets,proto3,oneof" json:"net_netfilter_nf_conntrack_buckets,omitempty"`
	FsInotifyMaxUserWatches        *int32  `protobuf:"varint,21,opt,name=fs_inotify_max_user_watches,json=fsInotifyMaxUserWatches,proto3,oneof" json:"fs_inotify_max_user_watches,omitempty"`
	FsFileMax                      *int32  `protobuf:"varint,22,opt,name=fs_file_max,json=fsFileMax,proto3,oneof" json:"fs_file_max,omitempty"`
	FsAioMaxNr                     *int32  `protobuf:"varint,23,opt,name=fs_aio_max_nr,json=fsAioMaxNr,proto3,oneof" json:"fs_aio_max_nr,omitempty"`
	FsNrOpen                       *int32  `protobuf:"varint,24,opt,name=fs_nr_open,json=fsNrOpen,proto3,oneof" json:"fs_nr_open,omitempty"`
	KernelThreadsMax               *int32  `protobuf:"varint,25,opt,name=kernel_threads_max,json=kernelThreadsMax,proto3,oneof" json:"kernel_threads_max,omitempty"`
	VmMaxMapCount                  *int32  `protobuf:"varint,26,opt,name=vm_max_map_count,json=vmMaxMapCount,proto3,oneof" json:"vm_max_map_count,omitempty"`
	VmSwappiness                   *int32  `protobuf:"varint,27,opt,name=vm_swappiness,json=vmSwappiness,proto3,oneof" json:"vm_swappiness,omitempty"`
	VmVfsCachePressure             *int32  `protobuf:"varint,28,opt,name=vm_vfs_cache_pressure,json=vmVfsCachePressure,proto3,oneof" json:"vm_vfs_cache_pressure,omitempty"`
}

func (x *SysctlConfig) Reset() {
	*x = SysctlConfig{}
	mi := &file_aksnodeconfig_v1_customlinuxosconfig_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SysctlConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SysctlConfig) ProtoMessage() {}

func (x *SysctlConfig) ProtoReflect() protoreflect.Message {
	mi := &file_aksnodeconfig_v1_customlinuxosconfig_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SysctlConfig.ProtoReflect.Descriptor instead.
func (*SysctlConfig) Descriptor() ([]byte, []int) {
	return file_aksnodeconfig_v1_customlinuxosconfig_proto_rawDescGZIP(), []int{1}
}

func (x *SysctlConfig) GetNetCoreSomaxconn() int32 {
	if x != nil && x.NetCoreSomaxconn != nil {
		return *x.NetCoreSomaxconn
	}
	return 0
}

func (x *SysctlConfig) GetNetCoreNetdevMaxBacklog() int32 {
	if x != nil && x.NetCoreNetdevMaxBacklog != nil {
		return *x.NetCoreNetdevMaxBacklog
	}
	return 0
}

func (x *SysctlConfig) GetNetCoreRmemDefault() int32 {
	if x != nil && x.NetCoreRmemDefault != nil {
		return *x.NetCoreRmemDefault
	}
	return 0
}

func (x *SysctlConfig) GetNetCoreRmemMax() int32 {
	if x != nil && x.NetCoreRmemMax != nil {
		return *x.NetCoreRmemMax
	}
	return 0
}

func (x *SysctlConfig) GetNetCoreWmemDefault() int32 {
	if x != nil && x.NetCoreWmemDefault != nil {
		return *x.NetCoreWmemDefault
	}
	return 0
}

func (x *SysctlConfig) GetNetCoreWmemMax() int32 {
	if x != nil && x.NetCoreWmemMax != nil {
		return *x.NetCoreWmemMax
	}
	return 0
}

func (x *SysctlConfig) GetNetCoreOptmemMax() int32 {
	if x != nil && x.NetCoreOptmemMax != nil {
		return *x.NetCoreOptmemMax
	}
	return 0
}

func (x *SysctlConfig) GetNetIpv4TcpMaxSynBacklog() int32 {
	if x != nil && x.NetIpv4TcpMaxSynBacklog != nil {
		return *x.NetIpv4TcpMaxSynBacklog
	}
	return 0
}

func (x *SysctlConfig) GetNetIpv4TcpMaxTwBuckets() int32 {
	if x != nil && x.NetIpv4TcpMaxTwBuckets != nil {
		return *x.NetIpv4TcpMaxTwBuckets
	}
	return 0
}

func (x *SysctlConfig) GetNetIpv4TcpFinTimeout() int32 {
	if x != nil && x.NetIpv4TcpFinTimeout != nil {
		return *x.NetIpv4TcpFinTimeout
	}
	return 0
}

func (x *SysctlConfig) GetNetIpv4TcpKeepaliveTime() int32 {
	if x != nil && x.NetIpv4TcpKeepaliveTime != nil {
		return *x.NetIpv4TcpKeepaliveTime
	}
	return 0
}

func (x *SysctlConfig) GetNetIpv4TcpKeepaliveProbes() int32 {
	if x != nil && x.NetIpv4TcpKeepaliveProbes != nil {
		return *x.NetIpv4TcpKeepaliveProbes
	}
	return 0
}

func (x *SysctlConfig) GetNetIpv4TcpkeepaliveIntvl() int32 {
	if x != nil && x.NetIpv4TcpkeepaliveIntvl != nil {
		return *x.NetIpv4TcpkeepaliveIntvl
	}
	return 0
}

func (x *SysctlConfig) GetNetIpv4TcpTwReuse() bool {
	if x != nil && x.NetIpv4TcpTwReuse != nil {
		return *x.NetIpv4TcpTwReuse
	}
	return false
}

func (x *SysctlConfig) GetNetIpv4IpLocalPortRange() string {
	if x != nil && x.NetIpv4IpLocalPortRange != nil {
		return *x.NetIpv4IpLocalPortRange
	}
	return ""
}

func (x *SysctlConfig) GetNetIpv4NeighDefaultGcThresh1() int32 {
	if x != nil && x.NetIpv4NeighDefaultGcThresh1 != nil {
		return *x.NetIpv4NeighDefaultGcThresh1
	}
	return 0
}

func (x *SysctlConfig) GetNetIpv4NeighDefaultGcThresh2() int32 {
	if x != nil && x.NetIpv4NeighDefaultGcThresh2 != nil {
		return *x.NetIpv4NeighDefaultGcThresh2
	}
	return 0
}

func (x *SysctlConfig) GetNetIpv4NeighDefaultGcThresh3() int32 {
	if x != nil && x.NetIpv4NeighDefaultGcThresh3 != nil {
		return *x.NetIpv4NeighDefaultGcThresh3
	}
	return 0
}

func (x *SysctlConfig) GetNetNetfilterNfConntrackMax() int32 {
	if x != nil && x.NetNetfilterNfConntrackMax != nil {
		return *x.NetNetfilterNfConntrackMax
	}
	return 0
}

func (x *SysctlConfig) GetNetNetfilterNfConntrackBuckets() int32 {
	if x != nil && x.NetNetfilterNfConntrackBuckets != nil {
		return *x.NetNetfilterNfConntrackBuckets
	}
	return 0
}

func (x *SysctlConfig) GetFsInotifyMaxUserWatches() int32 {
	if x != nil && x.FsInotifyMaxUserWatches != nil {
		return *x.FsInotifyMaxUserWatches
	}
	return 0
}

func (x *SysctlConfig) GetFsFileMax() int32 {
	if x != nil && x.FsFileMax != nil {
		return *x.FsFileMax
	}
	return 0
}

func (x *SysctlConfig) GetFsAioMaxNr() int32 {
	if x != nil && x.FsAioMaxNr != nil {
		return *x.FsAioMaxNr
	}
	return 0
}

func (x *SysctlConfig) GetFsNrOpen() int32 {
	if x != nil && x.FsNrOpen != nil {
		return *x.FsNrOpen
	}
	return 0
}

func (x *SysctlConfig) GetKernelThreadsMax() int32 {
	if x != nil && x.KernelThreadsMax != nil {
		return *x.KernelThreadsMax
	}
	return 0
}

func (x *SysctlConfig) GetVmMaxMapCount() int32 {
	if x != nil && x.VmMaxMapCount != nil {
		return *x.VmMaxMapCount
	}
	return 0
}

func (x *SysctlConfig) GetVmSwappiness() int32 {
	if x != nil && x.VmSwappiness != nil {
		return *x.VmSwappiness
	}
	return 0
}

func (x *SysctlConfig) GetVmVfsCachePressure() int32 {
	if x != nil && x.VmVfsCachePressure != nil {
		return *x.VmVfsCachePressure
	}
	return 0
}

type UlimitConfig struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// using optional here to allow detecting if the field is set or not (explicit presence in proto3)
	NoFile          *string `protobuf:"bytes,1,opt,name=no_file,json=noFile,proto3,oneof" json:"no_file,omitempty"`
	MaxLockedMemory *string `protobuf:"bytes,2,opt,name=max_locked_memory,json=maxLockedMemory,proto3,oneof" json:"max_locked_memory,omitempty"`
}

func (x *UlimitConfig) Reset() {
	*x = UlimitConfig{}
	mi := &file_aksnodeconfig_v1_customlinuxosconfig_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *UlimitConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UlimitConfig) ProtoMessage() {}

func (x *UlimitConfig) ProtoReflect() protoreflect.Message {
	mi := &file_aksnodeconfig_v1_customlinuxosconfig_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UlimitConfig.ProtoReflect.Descriptor instead.
func (*UlimitConfig) Descriptor() ([]byte, []int) {
	return file_aksnodeconfig_v1_customlinuxosconfig_proto_rawDescGZIP(), []int{2}
}

func (x *UlimitConfig) GetNoFile() string {
	if x != nil && x.NoFile != nil {
		return *x.NoFile
	}
	return ""
}

func (x *UlimitConfig) GetMaxLockedMemory() string {
	if x != nil && x.MaxLockedMemory != nil {
		return *x.MaxLockedMemory
	}
	return ""
}

var File_aksnodeconfig_v1_customlinuxosconfig_proto protoreflect.FileDescriptor

var file_aksnodeconfig_v1_customlinuxosconfig_proto_rawDesc = []byte{
	0x0a, 0x2a, 0x61, 0x6b, 0x73, 0x6e, 0x6f, 0x64, 0x65, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2f,
	0x76, 0x31, 0x2f, 0x63, 0x75, 0x73, 0x74, 0x6f, 0x6d, 0x6c, 0x69, 0x6e, 0x75, 0x78, 0x6f, 0x73,
	0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x10, 0x61, 0x6b,
	0x73, 0x6e, 0x6f, 0x64, 0x65, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x76, 0x31, 0x22, 0xe4,
	0x02, 0x0a, 0x13, 0x43, 0x75, 0x73, 0x74, 0x6f, 0x6d, 0x4c, 0x69, 0x6e, 0x75, 0x78, 0x4f, 0x53,
	0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x43, 0x0a, 0x0d, 0x73, 0x79, 0x73, 0x63, 0x74, 0x6c,
	0x5f, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1e, 0x2e,
	0x61, 0x6b, 0x73, 0x6e, 0x6f, 0x64, 0x65, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x76, 0x31,
	0x2e, 0x53, 0x79, 0x73, 0x63, 0x74, 0x6c, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x52, 0x0c, 0x73,
	0x79, 0x73, 0x63, 0x74, 0x6c, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x43, 0x0a, 0x0d, 0x75,
	0x6c, 0x69, 0x6d, 0x69, 0x74, 0x5f, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x1e, 0x2e, 0x61, 0x6b, 0x73, 0x6e, 0x6f, 0x64, 0x65, 0x63, 0x6f, 0x6e, 0x66,
	0x69, 0x67, 0x2e, 0x76, 0x31, 0x2e, 0x55, 0x6c, 0x69, 0x6d, 0x69, 0x74, 0x43, 0x6f, 0x6e, 0x66,
	0x69, 0x67, 0x52, 0x0c, 0x75, 0x6c, 0x69, 0x6d, 0x69, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67,
	0x12, 0x2c, 0x0a, 0x12, 0x65, 0x6e, 0x61, 0x62, 0x6c, 0x65, 0x5f, 0x73, 0x77, 0x61, 0x70, 0x5f,
	0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x18, 0x03, 0x20, 0x01, 0x28, 0x08, 0x52, 0x10, 0x65, 0x6e,
	0x61, 0x62, 0x6c, 0x65, 0x53, 0x77, 0x61, 0x70, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x24,
	0x0a, 0x0e, 0x73, 0x77, 0x61, 0x70, 0x5f, 0x66, 0x69, 0x6c, 0x65, 0x5f, 0x73, 0x69, 0x7a, 0x65,
	0x18, 0x04, 0x20, 0x01, 0x28, 0x05, 0x52, 0x0c, 0x73, 0x77, 0x61, 0x70, 0x46, 0x69, 0x6c, 0x65,
	0x53, 0x69, 0x7a, 0x65, 0x12, 0x40, 0x0a, 0x1c, 0x74, 0x72, 0x61, 0x6e, 0x73, 0x70, 0x61, 0x72,
	0x65, 0x6e, 0x74, 0x5f, 0x68, 0x75, 0x67, 0x65, 0x70, 0x61, 0x67, 0x65, 0x5f, 0x73, 0x75, 0x70,
	0x70, 0x6f, 0x72, 0x74, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x1a, 0x74, 0x72, 0x61, 0x6e,
	0x73, 0x70, 0x61, 0x72, 0x65, 0x6e, 0x74, 0x48, 0x75, 0x67, 0x65, 0x70, 0x61, 0x67, 0x65, 0x53,
	0x75, 0x70, 0x70, 0x6f, 0x72, 0x74, 0x12, 0x2d, 0x0a, 0x12, 0x74, 0x72, 0x61, 0x6e, 0x73, 0x70,
	0x61, 0x72, 0x65, 0x6e, 0x74, 0x5f, 0x64, 0x65, 0x66, 0x72, 0x61, 0x67, 0x18, 0x06, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x11, 0x74, 0x72, 0x61, 0x6e, 0x73, 0x70, 0x61, 0x72, 0x65, 0x6e, 0x74, 0x44,
	0x65, 0x66, 0x72, 0x61, 0x67, 0x22, 0x9d, 0x13, 0x0a, 0x0c, 0x53, 0x79, 0x73, 0x63, 0x74, 0x6c,
	0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x31, 0x0a, 0x12, 0x6e, 0x65, 0x74, 0x5f, 0x63, 0x6f,
	0x72, 0x65, 0x5f, 0x73, 0x6f, 0x6d, 0x61, 0x78, 0x63, 0x6f, 0x6e, 0x6e, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x05, 0x48, 0x00, 0x52, 0x10, 0x6e, 0x65, 0x74, 0x43, 0x6f, 0x72, 0x65, 0x53, 0x6f, 0x6d,
	0x61, 0x78, 0x63, 0x6f, 0x6e, 0x6e, 0x88, 0x01, 0x01, 0x12, 0x41, 0x0a, 0x1b, 0x6e, 0x65, 0x74,
	0x5f, 0x63, 0x6f, 0x72, 0x65, 0x5f, 0x6e, 0x65, 0x74, 0x64, 0x65, 0x76, 0x5f, 0x6d, 0x61, 0x78,
	0x5f, 0x62, 0x61, 0x63, 0x6b, 0x6c, 0x6f, 0x67, 0x18, 0x02, 0x20, 0x01, 0x28, 0x05, 0x48, 0x01,
	0x52, 0x17, 0x6e, 0x65, 0x74, 0x43, 0x6f, 0x72, 0x65, 0x4e, 0x65, 0x74, 0x64, 0x65, 0x76, 0x4d,
	0x61, 0x78, 0x42, 0x61, 0x63, 0x6b, 0x6c, 0x6f, 0x67, 0x88, 0x01, 0x01, 0x12, 0x36, 0x0a, 0x15,
	0x6e, 0x65, 0x74, 0x5f, 0x63, 0x6f, 0x72, 0x65, 0x5f, 0x72, 0x6d, 0x65, 0x6d, 0x5f, 0x64, 0x65,
	0x66, 0x61, 0x75, 0x6c, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x05, 0x48, 0x02, 0x52, 0x12, 0x6e,
	0x65, 0x74, 0x43, 0x6f, 0x72, 0x65, 0x52, 0x6d, 0x65, 0x6d, 0x44, 0x65, 0x66, 0x61, 0x75, 0x6c,
	0x74, 0x88, 0x01, 0x01, 0x12, 0x2e, 0x0a, 0x11, 0x6e, 0x65, 0x74, 0x5f, 0x63, 0x6f, 0x72, 0x65,
	0x5f, 0x72, 0x6d, 0x65, 0x6d, 0x5f, 0x6d, 0x61, 0x78, 0x18, 0x04, 0x20, 0x01, 0x28, 0x05, 0x48,
	0x03, 0x52, 0x0e, 0x6e, 0x65, 0x74, 0x43, 0x6f, 0x72, 0x65, 0x52, 0x6d, 0x65, 0x6d, 0x4d, 0x61,
	0x78, 0x88, 0x01, 0x01, 0x12, 0x36, 0x0a, 0x15, 0x6e, 0x65, 0x74, 0x5f, 0x63, 0x6f, 0x72, 0x65,
	0x5f, 0x77, 0x6d, 0x65, 0x6d, 0x5f, 0x64, 0x65, 0x66, 0x61, 0x75, 0x6c, 0x74, 0x18, 0x05, 0x20,
	0x01, 0x28, 0x05, 0x48, 0x04, 0x52, 0x12, 0x6e, 0x65, 0x74, 0x43, 0x6f, 0x72, 0x65, 0x57, 0x6d,
	0x65, 0x6d, 0x44, 0x65, 0x66, 0x61, 0x75, 0x6c, 0x74, 0x88, 0x01, 0x01, 0x12, 0x2e, 0x0a, 0x11,
	0x6e, 0x65, 0x74, 0x5f, 0x63, 0x6f, 0x72, 0x65, 0x5f, 0x77, 0x6d, 0x65, 0x6d, 0x5f, 0x6d, 0x61,
	0x78, 0x18, 0x06, 0x20, 0x01, 0x28, 0x05, 0x48, 0x05, 0x52, 0x0e, 0x6e, 0x65, 0x74, 0x43, 0x6f,
	0x72, 0x65, 0x57, 0x6d, 0x65, 0x6d, 0x4d, 0x61, 0x78, 0x88, 0x01, 0x01, 0x12, 0x32, 0x0a, 0x13,
	0x6e, 0x65, 0x74, 0x5f, 0x63, 0x6f, 0x72, 0x65, 0x5f, 0x6f, 0x70, 0x74, 0x6d, 0x65, 0x6d, 0x5f,
	0x6d, 0x61, 0x78, 0x18, 0x07, 0x20, 0x01, 0x28, 0x05, 0x48, 0x06, 0x52, 0x10, 0x6e, 0x65, 0x74,
	0x43, 0x6f, 0x72, 0x65, 0x4f, 0x70, 0x74, 0x6d, 0x65, 0x6d, 0x4d, 0x61, 0x78, 0x88, 0x01, 0x01,
	0x12, 0x42, 0x0a, 0x1c, 0x6e, 0x65, 0x74, 0x5f, 0x69, 0x70, 0x76, 0x34, 0x5f, 0x74, 0x63, 0x70,
	0x5f, 0x6d, 0x61, 0x78, 0x5f, 0x73, 0x79, 0x6e, 0x5f, 0x62, 0x61, 0x63, 0x6b, 0x6c, 0x6f, 0x67,
	0x18, 0x08, 0x20, 0x01, 0x28, 0x05, 0x48, 0x07, 0x52, 0x17, 0x6e, 0x65, 0x74, 0x49, 0x70, 0x76,
	0x34, 0x54, 0x63, 0x70, 0x4d, 0x61, 0x78, 0x53, 0x79, 0x6e, 0x42, 0x61, 0x63, 0x6b, 0x6c, 0x6f,
	0x67, 0x88, 0x01, 0x01, 0x12, 0x40, 0x0a, 0x1b, 0x6e, 0x65, 0x74, 0x5f, 0x69, 0x70, 0x76, 0x34,
	0x5f, 0x74, 0x63, 0x70, 0x5f, 0x6d, 0x61, 0x78, 0x5f, 0x74, 0x77, 0x5f, 0x62, 0x75, 0x63, 0x6b,
	0x65, 0x74, 0x73, 0x18, 0x09, 0x20, 0x01, 0x28, 0x05, 0x48, 0x08, 0x52, 0x16, 0x6e, 0x65, 0x74,
	0x49, 0x70, 0x76, 0x34, 0x54, 0x63, 0x70, 0x4d, 0x61, 0x78, 0x54, 0x77, 0x42, 0x75, 0x63, 0x6b,
	0x65, 0x74, 0x73, 0x88, 0x01, 0x01, 0x12, 0x3b, 0x0a, 0x18, 0x6e, 0x65, 0x74, 0x5f, 0x69, 0x70,
	0x76, 0x34, 0x5f, 0x74, 0x63, 0x70, 0x5f, 0x66, 0x69, 0x6e, 0x5f, 0x74, 0x69, 0x6d, 0x65, 0x6f,
	0x75, 0x74, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x05, 0x48, 0x09, 0x52, 0x14, 0x6e, 0x65, 0x74, 0x49,
	0x70, 0x76, 0x34, 0x54, 0x63, 0x70, 0x46, 0x69, 0x6e, 0x54, 0x69, 0x6d, 0x65, 0x6f, 0x75, 0x74,
	0x88, 0x01, 0x01, 0x12, 0x41, 0x0a, 0x1b, 0x6e, 0x65, 0x74, 0x5f, 0x69, 0x70, 0x76, 0x34, 0x5f,
	0x74, 0x63, 0x70, 0x5f, 0x6b, 0x65, 0x65, 0x70, 0x61, 0x6c, 0x69, 0x76, 0x65, 0x5f, 0x74, 0x69,
	0x6d, 0x65, 0x18, 0x0b, 0x20, 0x01, 0x28, 0x05, 0x48, 0x0a, 0x52, 0x17, 0x6e, 0x65, 0x74, 0x49,
	0x70, 0x76, 0x34, 0x54, 0x63, 0x70, 0x4b, 0x65, 0x65, 0x70, 0x61, 0x6c, 0x69, 0x76, 0x65, 0x54,
	0x69, 0x6d, 0x65, 0x88, 0x01, 0x01, 0x12, 0x45, 0x0a, 0x1d, 0x6e, 0x65, 0x74, 0x5f, 0x69, 0x70,
	0x76, 0x34, 0x5f, 0x74, 0x63, 0x70, 0x5f, 0x6b, 0x65, 0x65, 0x70, 0x61, 0x6c, 0x69, 0x76, 0x65,
	0x5f, 0x70, 0x72, 0x6f, 0x62, 0x65, 0x73, 0x18, 0x0c, 0x20, 0x01, 0x28, 0x05, 0x48, 0x0b, 0x52,
	0x19, 0x6e, 0x65, 0x74, 0x49, 0x70, 0x76, 0x34, 0x54, 0x63, 0x70, 0x4b, 0x65, 0x65, 0x70, 0x61,
	0x6c, 0x69, 0x76, 0x65, 0x50, 0x72, 0x6f, 0x62, 0x65, 0x73, 0x88, 0x01, 0x01, 0x12, 0x42, 0x0a,
	0x1b, 0x6e, 0x65, 0x74, 0x5f, 0x69, 0x70, 0x76, 0x34, 0x5f, 0x74, 0x63, 0x70, 0x6b, 0x65, 0x65,
	0x70, 0x61, 0x6c, 0x69, 0x76, 0x65, 0x5f, 0x69, 0x6e, 0x74, 0x76, 0x6c, 0x18, 0x0d, 0x20, 0x01,
	0x28, 0x05, 0x48, 0x0c, 0x52, 0x18, 0x6e, 0x65, 0x74, 0x49, 0x70, 0x76, 0x34, 0x54, 0x63, 0x70,
	0x6b, 0x65, 0x65, 0x70, 0x61, 0x6c, 0x69, 0x76, 0x65, 0x49, 0x6e, 0x74, 0x76, 0x6c, 0x88, 0x01,
	0x01, 0x12, 0x35, 0x0a, 0x15, 0x6e, 0x65, 0x74, 0x5f, 0x69, 0x70, 0x76, 0x34, 0x5f, 0x74, 0x63,
	0x70, 0x5f, 0x74, 0x77, 0x5f, 0x72, 0x65, 0x75, 0x73, 0x65, 0x18, 0x0e, 0x20, 0x01, 0x28, 0x08,
	0x48, 0x0d, 0x52, 0x11, 0x6e, 0x65, 0x74, 0x49, 0x70, 0x76, 0x34, 0x54, 0x63, 0x70, 0x54, 0x77,
	0x52, 0x65, 0x75, 0x73, 0x65, 0x88, 0x01, 0x01, 0x12, 0x42, 0x0a, 0x1c, 0x6e, 0x65, 0x74, 0x5f,
	0x69, 0x70, 0x76, 0x34, 0x5f, 0x69, 0x70, 0x5f, 0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x5f, 0x70, 0x6f,
	0x72, 0x74, 0x5f, 0x72, 0x61, 0x6e, 0x67, 0x65, 0x18, 0x0f, 0x20, 0x01, 0x28, 0x09, 0x48, 0x0e,
	0x52, 0x17, 0x6e, 0x65, 0x74, 0x49, 0x70, 0x76, 0x34, 0x49, 0x70, 0x4c, 0x6f, 0x63, 0x61, 0x6c,
	0x50, 0x6f, 0x72, 0x74, 0x52, 0x61, 0x6e, 0x67, 0x65, 0x88, 0x01, 0x01, 0x12, 0x4c, 0x0a, 0x21,
	0x6e, 0x65, 0x74, 0x5f, 0x69, 0x70, 0x76, 0x34, 0x5f, 0x6e, 0x65, 0x69, 0x67, 0x68, 0x5f, 0x64,
	0x65, 0x66, 0x61, 0x75, 0x6c, 0x74, 0x5f, 0x67, 0x63, 0x5f, 0x74, 0x68, 0x72, 0x65, 0x73, 0x68,
	0x31, 0x18, 0x10, 0x20, 0x01, 0x28, 0x05, 0x48, 0x0f, 0x52, 0x1c, 0x6e, 0x65, 0x74, 0x49, 0x70,
	0x76, 0x34, 0x4e, 0x65, 0x69, 0x67, 0x68, 0x44, 0x65, 0x66, 0x61, 0x75, 0x6c, 0x74, 0x47, 0x63,
	0x54, 0x68, 0x72, 0x65, 0x73, 0x68, 0x31, 0x88, 0x01, 0x01, 0x12, 0x4c, 0x0a, 0x21, 0x6e, 0x65,
	0x74, 0x5f, 0x69, 0x70, 0x76, 0x34, 0x5f, 0x6e, 0x65, 0x69, 0x67, 0x68, 0x5f, 0x64, 0x65, 0x66,
	0x61, 0x75, 0x6c, 0x74, 0x5f, 0x67, 0x63, 0x5f, 0x74, 0x68, 0x72, 0x65, 0x73, 0x68, 0x32, 0x18,
	0x11, 0x20, 0x01, 0x28, 0x05, 0x48, 0x10, 0x52, 0x1c, 0x6e, 0x65, 0x74, 0x49, 0x70, 0x76, 0x34,
	0x4e, 0x65, 0x69, 0x67, 0x68, 0x44, 0x65, 0x66, 0x61, 0x75, 0x6c, 0x74, 0x47, 0x63, 0x54, 0x68,
	0x72, 0x65, 0x73, 0x68, 0x32, 0x88, 0x01, 0x01, 0x12, 0x4c, 0x0a, 0x21, 0x6e, 0x65, 0x74, 0x5f,
	0x69, 0x70, 0x76, 0x34, 0x5f, 0x6e, 0x65, 0x69, 0x67, 0x68, 0x5f, 0x64, 0x65, 0x66, 0x61, 0x75,
	0x6c, 0x74, 0x5f, 0x67, 0x63, 0x5f, 0x74, 0x68, 0x72, 0x65, 0x73, 0x68, 0x33, 0x18, 0x12, 0x20,
	0x01, 0x28, 0x05, 0x48, 0x11, 0x52, 0x1c, 0x6e, 0x65, 0x74, 0x49, 0x70, 0x76, 0x34, 0x4e, 0x65,
	0x69, 0x67, 0x68, 0x44, 0x65, 0x66, 0x61, 0x75, 0x6c, 0x74, 0x47, 0x63, 0x54, 0x68, 0x72, 0x65,
	0x73, 0x68, 0x33, 0x88, 0x01, 0x01, 0x12, 0x47, 0x0a, 0x1e, 0x6e, 0x65, 0x74, 0x5f, 0x6e, 0x65,
	0x74, 0x66, 0x69, 0x6c, 0x74, 0x65, 0x72, 0x5f, 0x6e, 0x66, 0x5f, 0x63, 0x6f, 0x6e, 0x6e, 0x74,
	0x72, 0x61, 0x63, 0x6b, 0x5f, 0x6d, 0x61, 0x78, 0x18, 0x13, 0x20, 0x01, 0x28, 0x05, 0x48, 0x12,
	0x52, 0x1a, 0x6e, 0x65, 0x74, 0x4e, 0x65, 0x74, 0x66, 0x69, 0x6c, 0x74, 0x65, 0x72, 0x4e, 0x66,
	0x43, 0x6f, 0x6e, 0x6e, 0x74, 0x72, 0x61, 0x63, 0x6b, 0x4d, 0x61, 0x78, 0x88, 0x01, 0x01, 0x12,
	0x4f, 0x0a, 0x22, 0x6e, 0x65, 0x74, 0x5f, 0x6e, 0x65, 0x74, 0x66, 0x69, 0x6c, 0x74, 0x65, 0x72,
	0x5f, 0x6e, 0x66, 0x5f, 0x63, 0x6f, 0x6e, 0x6e, 0x74, 0x72, 0x61, 0x63, 0x6b, 0x5f, 0x62, 0x75,
	0x63, 0x6b, 0x65, 0x74, 0x73, 0x18, 0x14, 0x20, 0x01, 0x28, 0x05, 0x48, 0x13, 0x52, 0x1e, 0x6e,
	0x65, 0x74, 0x4e, 0x65, 0x74, 0x66, 0x69, 0x6c, 0x74, 0x65, 0x72, 0x4e, 0x66, 0x43, 0x6f, 0x6e,
	0x6e, 0x74, 0x72, 0x61, 0x63, 0x6b, 0x42, 0x75, 0x63, 0x6b, 0x65, 0x74, 0x73, 0x88, 0x01, 0x01,
	0x12, 0x41, 0x0a, 0x1b, 0x66, 0x73, 0x5f, 0x69, 0x6e, 0x6f, 0x74, 0x69, 0x66, 0x79, 0x5f, 0x6d,
	0x61, 0x78, 0x5f, 0x75, 0x73, 0x65, 0x72, 0x5f, 0x77, 0x61, 0x74, 0x63, 0x68, 0x65, 0x73, 0x18,
	0x15, 0x20, 0x01, 0x28, 0x05, 0x48, 0x14, 0x52, 0x17, 0x66, 0x73, 0x49, 0x6e, 0x6f, 0x74, 0x69,
	0x66, 0x79, 0x4d, 0x61, 0x78, 0x55, 0x73, 0x65, 0x72, 0x57, 0x61, 0x74, 0x63, 0x68, 0x65, 0x73,
	0x88, 0x01, 0x01, 0x12, 0x23, 0x0a, 0x0b, 0x66, 0x73, 0x5f, 0x66, 0x69, 0x6c, 0x65, 0x5f, 0x6d,
	0x61, 0x78, 0x18, 0x16, 0x20, 0x01, 0x28, 0x05, 0x48, 0x15, 0x52, 0x09, 0x66, 0x73, 0x46, 0x69,
	0x6c, 0x65, 0x4d, 0x61, 0x78, 0x88, 0x01, 0x01, 0x12, 0x26, 0x0a, 0x0d, 0x66, 0x73, 0x5f, 0x61,
	0x69, 0x6f, 0x5f, 0x6d, 0x61, 0x78, 0x5f, 0x6e, 0x72, 0x18, 0x17, 0x20, 0x01, 0x28, 0x05, 0x48,
	0x16, 0x52, 0x0a, 0x66, 0x73, 0x41, 0x69, 0x6f, 0x4d, 0x61, 0x78, 0x4e, 0x72, 0x88, 0x01, 0x01,
	0x12, 0x21, 0x0a, 0x0a, 0x66, 0x73, 0x5f, 0x6e, 0x72, 0x5f, 0x6f, 0x70, 0x65, 0x6e, 0x18, 0x18,
	0x20, 0x01, 0x28, 0x05, 0x48, 0x17, 0x52, 0x08, 0x66, 0x73, 0x4e, 0x72, 0x4f, 0x70, 0x65, 0x6e,
	0x88, 0x01, 0x01, 0x12, 0x31, 0x0a, 0x12, 0x6b, 0x65, 0x72, 0x6e, 0x65, 0x6c, 0x5f, 0x74, 0x68,
	0x72, 0x65, 0x61, 0x64, 0x73, 0x5f, 0x6d, 0x61, 0x78, 0x18, 0x19, 0x20, 0x01, 0x28, 0x05, 0x48,
	0x18, 0x52, 0x10, 0x6b, 0x65, 0x72, 0x6e, 0x65, 0x6c, 0x54, 0x68, 0x72, 0x65, 0x61, 0x64, 0x73,
	0x4d, 0x61, 0x78, 0x88, 0x01, 0x01, 0x12, 0x2c, 0x0a, 0x10, 0x76, 0x6d, 0x5f, 0x6d, 0x61, 0x78,
	0x5f, 0x6d, 0x61, 0x70, 0x5f, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x18, 0x1a, 0x20, 0x01, 0x28, 0x05,
	0x48, 0x19, 0x52, 0x0d, 0x76, 0x6d, 0x4d, 0x61, 0x78, 0x4d, 0x61, 0x70, 0x43, 0x6f, 0x75, 0x6e,
	0x74, 0x88, 0x01, 0x01, 0x12, 0x28, 0x0a, 0x0d, 0x76, 0x6d, 0x5f, 0x73, 0x77, 0x61, 0x70, 0x70,
	0x69, 0x6e, 0x65, 0x73, 0x73, 0x18, 0x1b, 0x20, 0x01, 0x28, 0x05, 0x48, 0x1a, 0x52, 0x0c, 0x76,
	0x6d, 0x53, 0x77, 0x61, 0x70, 0x70, 0x69, 0x6e, 0x65, 0x73, 0x73, 0x88, 0x01, 0x01, 0x12, 0x36,
	0x0a, 0x15, 0x76, 0x6d, 0x5f, 0x76, 0x66, 0x73, 0x5f, 0x63, 0x61, 0x63, 0x68, 0x65, 0x5f, 0x70,
	0x72, 0x65, 0x73, 0x73, 0x75, 0x72, 0x65, 0x18, 0x1c, 0x20, 0x01, 0x28, 0x05, 0x48, 0x1b, 0x52,
	0x12, 0x76, 0x6d, 0x56, 0x66, 0x73, 0x43, 0x61, 0x63, 0x68, 0x65, 0x50, 0x72, 0x65, 0x73, 0x73,
	0x75, 0x72, 0x65, 0x88, 0x01, 0x01, 0x42, 0x15, 0x0a, 0x13, 0x5f, 0x6e, 0x65, 0x74, 0x5f, 0x63,
	0x6f, 0x72, 0x65, 0x5f, 0x73, 0x6f, 0x6d, 0x61, 0x78, 0x63, 0x6f, 0x6e, 0x6e, 0x42, 0x1e, 0x0a,
	0x1c, 0x5f, 0x6e, 0x65, 0x74, 0x5f, 0x63, 0x6f, 0x72, 0x65, 0x5f, 0x6e, 0x65, 0x74, 0x64, 0x65,
	0x76, 0x5f, 0x6d, 0x61, 0x78, 0x5f, 0x62, 0x61, 0x63, 0x6b, 0x6c, 0x6f, 0x67, 0x42, 0x18, 0x0a,
	0x16, 0x5f, 0x6e, 0x65, 0x74, 0x5f, 0x63, 0x6f, 0x72, 0x65, 0x5f, 0x72, 0x6d, 0x65, 0x6d, 0x5f,
	0x64, 0x65, 0x66, 0x61, 0x75, 0x6c, 0x74, 0x42, 0x14, 0x0a, 0x12, 0x5f, 0x6e, 0x65, 0x74, 0x5f,
	0x63, 0x6f, 0x72, 0x65, 0x5f, 0x72, 0x6d, 0x65, 0x6d, 0x5f, 0x6d, 0x61, 0x78, 0x42, 0x18, 0x0a,
	0x16, 0x5f, 0x6e, 0x65, 0x74, 0x5f, 0x63, 0x6f, 0x72, 0x65, 0x5f, 0x77, 0x6d, 0x65, 0x6d, 0x5f,
	0x64, 0x65, 0x66, 0x61, 0x75, 0x6c, 0x74, 0x42, 0x14, 0x0a, 0x12, 0x5f, 0x6e, 0x65, 0x74, 0x5f,
	0x63, 0x6f, 0x72, 0x65, 0x5f, 0x77, 0x6d, 0x65, 0x6d, 0x5f, 0x6d, 0x61, 0x78, 0x42, 0x16, 0x0a,
	0x14, 0x5f, 0x6e, 0x65, 0x74, 0x5f, 0x63, 0x6f, 0x72, 0x65, 0x5f, 0x6f, 0x70, 0x74, 0x6d, 0x65,
	0x6d, 0x5f, 0x6d, 0x61, 0x78, 0x42, 0x1f, 0x0a, 0x1d, 0x5f, 0x6e, 0x65, 0x74, 0x5f, 0x69, 0x70,
	0x76, 0x34, 0x5f, 0x74, 0x63, 0x70, 0x5f, 0x6d, 0x61, 0x78, 0x5f, 0x73, 0x79, 0x6e, 0x5f, 0x62,
	0x61, 0x63, 0x6b, 0x6c, 0x6f, 0x67, 0x42, 0x1e, 0x0a, 0x1c, 0x5f, 0x6e, 0x65, 0x74, 0x5f, 0x69,
	0x70, 0x76, 0x34, 0x5f, 0x74, 0x63, 0x70, 0x5f, 0x6d, 0x61, 0x78, 0x5f, 0x74, 0x77, 0x5f, 0x62,
	0x75, 0x63, 0x6b, 0x65, 0x74, 0x73, 0x42, 0x1b, 0x0a, 0x19, 0x5f, 0x6e, 0x65, 0x74, 0x5f, 0x69,
	0x70, 0x76, 0x34, 0x5f, 0x74, 0x63, 0x70, 0x5f, 0x66, 0x69, 0x6e, 0x5f, 0x74, 0x69, 0x6d, 0x65,
	0x6f, 0x75, 0x74, 0x42, 0x1e, 0x0a, 0x1c, 0x5f, 0x6e, 0x65, 0x74, 0x5f, 0x69, 0x70, 0x76, 0x34,
	0x5f, 0x74, 0x63, 0x70, 0x5f, 0x6b, 0x65, 0x65, 0x70, 0x61, 0x6c, 0x69, 0x76, 0x65, 0x5f, 0x74,
	0x69, 0x6d, 0x65, 0x42, 0x20, 0x0a, 0x1e, 0x5f, 0x6e, 0x65, 0x74, 0x5f, 0x69, 0x70, 0x76, 0x34,
	0x5f, 0x74, 0x63, 0x70, 0x5f, 0x6b, 0x65, 0x65, 0x70, 0x61, 0x6c, 0x69, 0x76, 0x65, 0x5f, 0x70,
	0x72, 0x6f, 0x62, 0x65, 0x73, 0x42, 0x1e, 0x0a, 0x1c, 0x5f, 0x6e, 0x65, 0x74, 0x5f, 0x69, 0x70,
	0x76, 0x34, 0x5f, 0x74, 0x63, 0x70, 0x6b, 0x65, 0x65, 0x70, 0x61, 0x6c, 0x69, 0x76, 0x65, 0x5f,
	0x69, 0x6e, 0x74, 0x76, 0x6c, 0x42, 0x18, 0x0a, 0x16, 0x5f, 0x6e, 0x65, 0x74, 0x5f, 0x69, 0x70,
	0x76, 0x34, 0x5f, 0x74, 0x63, 0x70, 0x5f, 0x74, 0x77, 0x5f, 0x72, 0x65, 0x75, 0x73, 0x65, 0x42,
	0x1f, 0x0a, 0x1d, 0x5f, 0x6e, 0x65, 0x74, 0x5f, 0x69, 0x70, 0x76, 0x34, 0x5f, 0x69, 0x70, 0x5f,
	0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x5f, 0x70, 0x6f, 0x72, 0x74, 0x5f, 0x72, 0x61, 0x6e, 0x67, 0x65,
	0x42, 0x24, 0x0a, 0x22, 0x5f, 0x6e, 0x65, 0x74, 0x5f, 0x69, 0x70, 0x76, 0x34, 0x5f, 0x6e, 0x65,
	0x69, 0x67, 0x68, 0x5f, 0x64, 0x65, 0x66, 0x61, 0x75, 0x6c, 0x74, 0x5f, 0x67, 0x63, 0x5f, 0x74,
	0x68, 0x72, 0x65, 0x73, 0x68, 0x31, 0x42, 0x24, 0x0a, 0x22, 0x5f, 0x6e, 0x65, 0x74, 0x5f, 0x69,
	0x70, 0x76, 0x34, 0x5f, 0x6e, 0x65, 0x69, 0x67, 0x68, 0x5f, 0x64, 0x65, 0x66, 0x61, 0x75, 0x6c,
	0x74, 0x5f, 0x67, 0x63, 0x5f, 0x74, 0x68, 0x72, 0x65, 0x73, 0x68, 0x32, 0x42, 0x24, 0x0a, 0x22,
	0x5f, 0x6e, 0x65, 0x74, 0x5f, 0x69, 0x70, 0x76, 0x34, 0x5f, 0x6e, 0x65, 0x69, 0x67, 0x68, 0x5f,
	0x64, 0x65, 0x66, 0x61, 0x75, 0x6c, 0x74, 0x5f, 0x67, 0x63, 0x5f, 0x74, 0x68, 0x72, 0x65, 0x73,
	0x68, 0x33, 0x42, 0x21, 0x0a, 0x1f, 0x5f, 0x6e, 0x65, 0x74, 0x5f, 0x6e, 0x65, 0x74, 0x66, 0x69,
	0x6c, 0x74, 0x65, 0x72, 0x5f, 0x6e, 0x66, 0x5f, 0x63, 0x6f, 0x6e, 0x6e, 0x74, 0x72, 0x61, 0x63,
	0x6b, 0x5f, 0x6d, 0x61, 0x78, 0x42, 0x25, 0x0a, 0x23, 0x5f, 0x6e, 0x65, 0x74, 0x5f, 0x6e, 0x65,
	0x74, 0x66, 0x69, 0x6c, 0x74, 0x65, 0x72, 0x5f, 0x6e, 0x66, 0x5f, 0x63, 0x6f, 0x6e, 0x6e, 0x74,
	0x72, 0x61, 0x63, 0x6b, 0x5f, 0x62, 0x75, 0x63, 0x6b, 0x65, 0x74, 0x73, 0x42, 0x1e, 0x0a, 0x1c,
	0x5f, 0x66, 0x73, 0x5f, 0x69, 0x6e, 0x6f, 0x74, 0x69, 0x66, 0x79, 0x5f, 0x6d, 0x61, 0x78, 0x5f,
	0x75, 0x73, 0x65, 0x72, 0x5f, 0x77, 0x61, 0x74, 0x63, 0x68, 0x65, 0x73, 0x42, 0x0e, 0x0a, 0x0c,
	0x5f, 0x66, 0x73, 0x5f, 0x66, 0x69, 0x6c, 0x65, 0x5f, 0x6d, 0x61, 0x78, 0x42, 0x10, 0x0a, 0x0e,
	0x5f, 0x66, 0x73, 0x5f, 0x61, 0x69, 0x6f, 0x5f, 0x6d, 0x61, 0x78, 0x5f, 0x6e, 0x72, 0x42, 0x0d,
	0x0a, 0x0b, 0x5f, 0x66, 0x73, 0x5f, 0x6e, 0x72, 0x5f, 0x6f, 0x70, 0x65, 0x6e, 0x42, 0x15, 0x0a,
	0x13, 0x5f, 0x6b, 0x65, 0x72, 0x6e, 0x65, 0x6c, 0x5f, 0x74, 0x68, 0x72, 0x65, 0x61, 0x64, 0x73,
	0x5f, 0x6d, 0x61, 0x78, 0x42, 0x13, 0x0a, 0x11, 0x5f, 0x76, 0x6d, 0x5f, 0x6d, 0x61, 0x78, 0x5f,
	0x6d, 0x61, 0x70, 0x5f, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x42, 0x10, 0x0a, 0x0e, 0x5f, 0x76, 0x6d,
	0x5f, 0x73, 0x77, 0x61, 0x70, 0x70, 0x69, 0x6e, 0x65, 0x73, 0x73, 0x42, 0x18, 0x0a, 0x16, 0x5f,
	0x76, 0x6d, 0x5f, 0x76, 0x66, 0x73, 0x5f, 0x63, 0x61, 0x63, 0x68, 0x65, 0x5f, 0x70, 0x72, 0x65,
	0x73, 0x73, 0x75, 0x72, 0x65, 0x22, 0x7f, 0x0a, 0x0c, 0x55, 0x6c, 0x69, 0x6d, 0x69, 0x74, 0x43,
	0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x1c, 0x0a, 0x07, 0x6e, 0x6f, 0x5f, 0x66, 0x69, 0x6c, 0x65,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52, 0x06, 0x6e, 0x6f, 0x46, 0x69, 0x6c, 0x65,
	0x88, 0x01, 0x01, 0x12, 0x2f, 0x0a, 0x11, 0x6d, 0x61, 0x78, 0x5f, 0x6c, 0x6f, 0x63, 0x6b, 0x65,
	0x64, 0x5f, 0x6d, 0x65, 0x6d, 0x6f, 0x72, 0x79, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x48, 0x01,
	0x52, 0x0f, 0x6d, 0x61, 0x78, 0x4c, 0x6f, 0x63, 0x6b, 0x65, 0x64, 0x4d, 0x65, 0x6d, 0x6f, 0x72,
	0x79, 0x88, 0x01, 0x01, 0x42, 0x0a, 0x0a, 0x08, 0x5f, 0x6e, 0x6f, 0x5f, 0x66, 0x69, 0x6c, 0x65,
	0x42, 0x14, 0x0a, 0x12, 0x5f, 0x6d, 0x61, 0x78, 0x5f, 0x6c, 0x6f, 0x63, 0x6b, 0x65, 0x64, 0x5f,
	0x6d, 0x65, 0x6d, 0x6f, 0x72, 0x79, 0x42, 0xd3, 0x01, 0x0a, 0x14, 0x63, 0x6f, 0x6d, 0x2e, 0x61,
	0x6b, 0x73, 0x6e, 0x6f, 0x64, 0x65, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x76, 0x31, 0x42,
	0x18, 0x43, 0x75, 0x73, 0x74, 0x6f, 0x6d, 0x6c, 0x69, 0x6e, 0x75, 0x78, 0x6f, 0x73, 0x63, 0x6f,
	0x6e, 0x66, 0x69, 0x67, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x40, 0x67, 0x69, 0x74,
	0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x41, 0x7a, 0x75, 0x72, 0x65, 0x2f, 0x41, 0x67,
	0x65, 0x6e, 0x74, 0x42, 0x61, 0x6b, 0x65, 0x72, 0x2f, 0x70, 0x6b, 0x67, 0x2f, 0x61, 0x6b, 0x73,
	0x6e, 0x6f, 0x64, 0x65, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2f, 0x76, 0x31, 0x3b, 0x61, 0x6b,
	0x73, 0x6e, 0x6f, 0x64, 0x65, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x76, 0x31, 0xa2, 0x02, 0x03,
	0x41, 0x58, 0x58, 0xaa, 0x02, 0x10, 0x41, 0x6b, 0x73, 0x6e, 0x6f, 0x64, 0x65, 0x63, 0x6f, 0x6e,
	0x66, 0x69, 0x67, 0x2e, 0x56, 0x31, 0xca, 0x02, 0x10, 0x41, 0x6b, 0x73, 0x6e, 0x6f, 0x64, 0x65,
	0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x5c, 0x56, 0x31, 0xe2, 0x02, 0x1c, 0x41, 0x6b, 0x73, 0x6e,
	0x6f, 0x64, 0x65, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x5c, 0x56, 0x31, 0x5c, 0x47, 0x50, 0x42,
	0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0xea, 0x02, 0x11, 0x41, 0x6b, 0x73, 0x6e, 0x6f,
	0x64, 0x65, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x3a, 0x3a, 0x56, 0x31, 0x62, 0x06, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_aksnodeconfig_v1_customlinuxosconfig_proto_rawDescOnce sync.Once
	file_aksnodeconfig_v1_customlinuxosconfig_proto_rawDescData = file_aksnodeconfig_v1_customlinuxosconfig_proto_rawDesc
)

func file_aksnodeconfig_v1_customlinuxosconfig_proto_rawDescGZIP() []byte {
	file_aksnodeconfig_v1_customlinuxosconfig_proto_rawDescOnce.Do(func() {
		file_aksnodeconfig_v1_customlinuxosconfig_proto_rawDescData = protoimpl.X.CompressGZIP(file_aksnodeconfig_v1_customlinuxosconfig_proto_rawDescData)
	})
	return file_aksnodeconfig_v1_customlinuxosconfig_proto_rawDescData
}

var file_aksnodeconfig_v1_customlinuxosconfig_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_aksnodeconfig_v1_customlinuxosconfig_proto_goTypes = []any{
	(*CustomLinuxOSConfig)(nil), // 0: aksnodeconfig.v1.CustomLinuxOSConfig
	(*SysctlConfig)(nil),        // 1: aksnodeconfig.v1.SysctlConfig
	(*UlimitConfig)(nil),        // 2: aksnodeconfig.v1.UlimitConfig
}
var file_aksnodeconfig_v1_customlinuxosconfig_proto_depIdxs = []int32{
	1, // 0: aksnodeconfig.v1.CustomLinuxOSConfig.sysctl_config:type_name -> aksnodeconfig.v1.SysctlConfig
	2, // 1: aksnodeconfig.v1.CustomLinuxOSConfig.ulimit_config:type_name -> aksnodeconfig.v1.UlimitConfig
	2, // [2:2] is the sub-list for method output_type
	2, // [2:2] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_aksnodeconfig_v1_customlinuxosconfig_proto_init() }
func file_aksnodeconfig_v1_customlinuxosconfig_proto_init() {
	if File_aksnodeconfig_v1_customlinuxosconfig_proto != nil {
		return
	}
	file_aksnodeconfig_v1_customlinuxosconfig_proto_msgTypes[1].OneofWrappers = []any{}
	file_aksnodeconfig_v1_customlinuxosconfig_proto_msgTypes[2].OneofWrappers = []any{}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_aksnodeconfig_v1_customlinuxosconfig_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_aksnodeconfig_v1_customlinuxosconfig_proto_goTypes,
		DependencyIndexes: file_aksnodeconfig_v1_customlinuxosconfig_proto_depIdxs,
		MessageInfos:      file_aksnodeconfig_v1_customlinuxosconfig_proto_msgTypes,
	}.Build()
	File_aksnodeconfig_v1_customlinuxosconfig_proto = out.File
	file_aksnodeconfig_v1_customlinuxosconfig_proto_rawDesc = nil
	file_aksnodeconfig_v1_customlinuxosconfig_proto_goTypes = nil
	file_aksnodeconfig_v1_customlinuxosconfig_proto_depIdxs = nil
}
