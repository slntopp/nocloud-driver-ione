//
//Copyright © 2021-2022 Nikita Ivanovski info@slnt-opp.xyz
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        v3.18.1
// source: pkg/edge/proto/edge.proto

package proto

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	structpb "google.golang.org/protobuf/types/known/structpb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type InstanceState struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Instance string                     `protobuf:"bytes,1,opt,name=instance,proto3" json:"instance,omitempty"`
	Data     map[string]*structpb.Value `protobuf:"bytes,2,rep,name=data,proto3" json:"data,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
}

func (x *InstanceState) Reset() {
	*x = InstanceState{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_edge_proto_edge_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *InstanceState) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*InstanceState) ProtoMessage() {}

func (x *InstanceState) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_edge_proto_edge_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use InstanceState.ProtoReflect.Descriptor instead.
func (*InstanceState) Descriptor() ([]byte, []int) {
	return file_pkg_edge_proto_edge_proto_rawDescGZIP(), []int{0}
}

func (x *InstanceState) GetInstance() string {
	if x != nil {
		return x.Instance
	}
	return ""
}

func (x *InstanceState) GetData() map[string]*structpb.Value {
	if x != nil {
		return x.Data
	}
	return nil
}

type PostResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *PostResponse) Reset() {
	*x = PostResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_edge_proto_edge_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PostResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PostResponse) ProtoMessage() {}

func (x *PostResponse) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_edge_proto_edge_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PostResponse.ProtoReflect.Descriptor instead.
func (*PostResponse) Descriptor() ([]byte, []int) {
	return file_pkg_edge_proto_edge_proto_rawDescGZIP(), []int{1}
}

var File_pkg_edge_proto_edge_proto protoreflect.FileDescriptor

var file_pkg_edge_proto_edge_proto_rawDesc = []byte{
	0x0a, 0x19, 0x70, 0x6b, 0x67, 0x2f, 0x65, 0x64, 0x67, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x2f, 0x65, 0x64, 0x67, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x18, 0x6e, 0x6f, 0x63,
	0x6c, 0x6f, 0x75, 0x64, 0x5f, 0x64, 0x72, 0x69, 0x76, 0x65, 0x72, 0x5f, 0x69, 0x6f, 0x6e, 0x65,
	0x2e, 0x65, 0x64, 0x67, 0x65, 0x1a, 0x1c, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x73, 0x74, 0x72, 0x75, 0x63, 0x74, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x22, 0xc3, 0x01, 0x0a, 0x0d, 0x49, 0x6e, 0x73, 0x74, 0x61, 0x6e, 0x63, 0x65,
	0x53, 0x74, 0x61, 0x74, 0x65, 0x12, 0x1a, 0x0a, 0x08, 0x69, 0x6e, 0x73, 0x74, 0x61, 0x6e, 0x63,
	0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x69, 0x6e, 0x73, 0x74, 0x61, 0x6e, 0x63,
	0x65, 0x12, 0x45, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32,
	0x31, 0x2e, 0x6e, 0x6f, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x5f, 0x64, 0x72, 0x69, 0x76, 0x65, 0x72,
	0x5f, 0x69, 0x6f, 0x6e, 0x65, 0x2e, 0x65, 0x64, 0x67, 0x65, 0x2e, 0x49, 0x6e, 0x73, 0x74, 0x61,
	0x6e, 0x63, 0x65, 0x53, 0x74, 0x61, 0x74, 0x65, 0x2e, 0x44, 0x61, 0x74, 0x61, 0x45, 0x6e, 0x74,
	0x72, 0x79, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x1a, 0x4f, 0x0a, 0x09, 0x44, 0x61, 0x74, 0x61,
	0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x2c, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x52, 0x05,
	0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x22, 0x0e, 0x0a, 0x0c, 0x50, 0x6f, 0x73,
	0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x32, 0x73, 0x0a, 0x0b, 0x45, 0x64, 0x67,
	0x65, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x64, 0x0a, 0x11, 0x50, 0x6f, 0x73, 0x74,
	0x49, 0x6e, 0x73, 0x74, 0x61, 0x6e, 0x63, 0x65, 0x53, 0x74, 0x61, 0x74, 0x65, 0x12, 0x27, 0x2e,
	0x6e, 0x6f, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x5f, 0x64, 0x72, 0x69, 0x76, 0x65, 0x72, 0x5f, 0x69,
	0x6f, 0x6e, 0x65, 0x2e, 0x65, 0x64, 0x67, 0x65, 0x2e, 0x49, 0x6e, 0x73, 0x74, 0x61, 0x6e, 0x63,
	0x65, 0x53, 0x74, 0x61, 0x74, 0x65, 0x1a, 0x26, 0x2e, 0x6e, 0x6f, 0x63, 0x6c, 0x6f, 0x75, 0x64,
	0x5f, 0x64, 0x72, 0x69, 0x76, 0x65, 0x72, 0x5f, 0x69, 0x6f, 0x6e, 0x65, 0x2e, 0x65, 0x64, 0x67,
	0x65, 0x2e, 0x50, 0x6f, 0x73, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x42, 0xd9,
	0x01, 0x0a, 0x1c, 0x63, 0x6f, 0x6d, 0x2e, 0x6e, 0x6f, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x5f, 0x64,
	0x72, 0x69, 0x76, 0x65, 0x72, 0x5f, 0x69, 0x6f, 0x6e, 0x65, 0x2e, 0x65, 0x64, 0x67, 0x65, 0x42,
	0x09, 0x45, 0x64, 0x67, 0x65, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x35, 0x67, 0x69,
	0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x73, 0x6c, 0x6e, 0x74, 0x6f, 0x70, 0x70,
	0x2f, 0x6e, 0x6f, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x2d, 0x64, 0x72, 0x69, 0x76, 0x65, 0x72, 0x2d,
	0x69, 0x6f, 0x6e, 0x65, 0x2f, 0x70, 0x6b, 0x67, 0x2f, 0x65, 0x64, 0x67, 0x65, 0x2f, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0xa2, 0x02, 0x03, 0x4e, 0x45, 0x58, 0xaa, 0x02, 0x16, 0x4e, 0x6f, 0x63, 0x6c,
	0x6f, 0x75, 0x64, 0x44, 0x72, 0x69, 0x76, 0x65, 0x72, 0x49, 0x6f, 0x6e, 0x65, 0x2e, 0x45, 0x64,
	0x67, 0x65, 0xca, 0x02, 0x16, 0x4e, 0x6f, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x44, 0x72, 0x69, 0x76,
	0x65, 0x72, 0x49, 0x6f, 0x6e, 0x65, 0x5c, 0x45, 0x64, 0x67, 0x65, 0xe2, 0x02, 0x22, 0x4e, 0x6f,
	0x63, 0x6c, 0x6f, 0x75, 0x64, 0x44, 0x72, 0x69, 0x76, 0x65, 0x72, 0x49, 0x6f, 0x6e, 0x65, 0x5c,
	0x45, 0x64, 0x67, 0x65, 0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61,
	0xea, 0x02, 0x17, 0x4e, 0x6f, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x44, 0x72, 0x69, 0x76, 0x65, 0x72,
	0x49, 0x6f, 0x6e, 0x65, 0x3a, 0x3a, 0x45, 0x64, 0x67, 0x65, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x33,
}

var (
	file_pkg_edge_proto_edge_proto_rawDescOnce sync.Once
	file_pkg_edge_proto_edge_proto_rawDescData = file_pkg_edge_proto_edge_proto_rawDesc
)

func file_pkg_edge_proto_edge_proto_rawDescGZIP() []byte {
	file_pkg_edge_proto_edge_proto_rawDescOnce.Do(func() {
		file_pkg_edge_proto_edge_proto_rawDescData = protoimpl.X.CompressGZIP(file_pkg_edge_proto_edge_proto_rawDescData)
	})
	return file_pkg_edge_proto_edge_proto_rawDescData
}

var file_pkg_edge_proto_edge_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_pkg_edge_proto_edge_proto_goTypes = []interface{}{
	(*InstanceState)(nil),  // 0: nocloud_driver_ione.edge.InstanceState
	(*PostResponse)(nil),   // 1: nocloud_driver_ione.edge.PostResponse
	nil,                    // 2: nocloud_driver_ione.edge.InstanceState.DataEntry
	(*structpb.Value)(nil), // 3: google.protobuf.Value
}
var file_pkg_edge_proto_edge_proto_depIdxs = []int32{
	2, // 0: nocloud_driver_ione.edge.InstanceState.data:type_name -> nocloud_driver_ione.edge.InstanceState.DataEntry
	3, // 1: nocloud_driver_ione.edge.InstanceState.DataEntry.value:type_name -> google.protobuf.Value
	0, // 2: nocloud_driver_ione.edge.EdgeService.PostInstanceState:input_type -> nocloud_driver_ione.edge.InstanceState
	1, // 3: nocloud_driver_ione.edge.EdgeService.PostInstanceState:output_type -> nocloud_driver_ione.edge.PostResponse
	3, // [3:4] is the sub-list for method output_type
	2, // [2:3] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_pkg_edge_proto_edge_proto_init() }
func file_pkg_edge_proto_edge_proto_init() {
	if File_pkg_edge_proto_edge_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_pkg_edge_proto_edge_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*InstanceState); i {
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
		file_pkg_edge_proto_edge_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PostResponse); i {
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
			RawDescriptor: file_pkg_edge_proto_edge_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_pkg_edge_proto_edge_proto_goTypes,
		DependencyIndexes: file_pkg_edge_proto_edge_proto_depIdxs,
		MessageInfos:      file_pkg_edge_proto_edge_proto_msgTypes,
	}.Build()
	File_pkg_edge_proto_edge_proto = out.File
	file_pkg_edge_proto_edge_proto_rawDesc = nil
	file_pkg_edge_proto_edge_proto_goTypes = nil
	file_pkg_edge_proto_edge_proto_depIdxs = nil
}
