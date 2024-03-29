// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.19.6
// source: Blobs.proto

package contester_proto

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

type Blob_CompressionInfo_CompressionType int32

const (
	Blob_CompressionInfo_METHOD_NONE Blob_CompressionInfo_CompressionType = 0
	Blob_CompressionInfo_METHOD_ZLIB Blob_CompressionInfo_CompressionType = 1
)

// Enum value maps for Blob_CompressionInfo_CompressionType.
var (
	Blob_CompressionInfo_CompressionType_name = map[int32]string{
		0: "METHOD_NONE",
		1: "METHOD_ZLIB",
	}
	Blob_CompressionInfo_CompressionType_value = map[string]int32{
		"METHOD_NONE": 0,
		"METHOD_ZLIB": 1,
	}
)

func (x Blob_CompressionInfo_CompressionType) Enum() *Blob_CompressionInfo_CompressionType {
	p := new(Blob_CompressionInfo_CompressionType)
	*p = x
	return p
}

func (x Blob_CompressionInfo_CompressionType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Blob_CompressionInfo_CompressionType) Descriptor() protoreflect.EnumDescriptor {
	return file_Blobs_proto_enumTypes[0].Descriptor()
}

func (Blob_CompressionInfo_CompressionType) Type() protoreflect.EnumType {
	return &file_Blobs_proto_enumTypes[0]
}

func (x Blob_CompressionInfo_CompressionType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Blob_CompressionInfo_CompressionType.Descriptor instead.
func (Blob_CompressionInfo_CompressionType) EnumDescriptor() ([]byte, []int) {
	return file_Blobs_proto_rawDescGZIP(), []int{0, 0, 0}
}

type Blob struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Data        []byte                `protobuf:"bytes,1,opt,name=data,proto3" json:"data,omitempty"`
	Compression *Blob_CompressionInfo `protobuf:"bytes,2,opt,name=compression,proto3" json:"compression,omitempty"`
	Sha1        []byte                `protobuf:"bytes,3,opt,name=sha1,proto3" json:"sha1,omitempty"`
}

func (x *Blob) Reset() {
	*x = Blob{}
	if protoimpl.UnsafeEnabled {
		mi := &file_Blobs_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Blob) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Blob) ProtoMessage() {}

func (x *Blob) ProtoReflect() protoreflect.Message {
	mi := &file_Blobs_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Blob.ProtoReflect.Descriptor instead.
func (*Blob) Descriptor() ([]byte, []int) {
	return file_Blobs_proto_rawDescGZIP(), []int{0}
}

func (x *Blob) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

func (x *Blob) GetCompression() *Blob_CompressionInfo {
	if x != nil {
		return x.Compression
	}
	return nil
}

func (x *Blob) GetSha1() []byte {
	if x != nil {
		return x.Sha1
	}
	return nil
}

type Module struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name string `protobuf:"bytes,3,opt,name=name,proto3" json:"name,omitempty"`
	Data *Blob  `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
	Type string `protobuf:"bytes,1,opt,name=type,proto3" json:"type,omitempty"`
}

func (x *Module) Reset() {
	*x = Module{}
	if protoimpl.UnsafeEnabled {
		mi := &file_Blobs_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Module) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Module) ProtoMessage() {}

func (x *Module) ProtoReflect() protoreflect.Message {
	mi := &file_Blobs_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Module.ProtoReflect.Descriptor instead.
func (*Module) Descriptor() ([]byte, []int) {
	return file_Blobs_proto_rawDescGZIP(), []int{1}
}

func (x *Module) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Module) GetData() *Blob {
	if x != nil {
		return x.Data
	}
	return nil
}

func (x *Module) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

type FileBlob struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Data *Blob  `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
}

func (x *FileBlob) Reset() {
	*x = FileBlob{}
	if protoimpl.UnsafeEnabled {
		mi := &file_Blobs_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *FileBlob) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*FileBlob) ProtoMessage() {}

func (x *FileBlob) ProtoReflect() protoreflect.Message {
	mi := &file_Blobs_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use FileBlob.ProtoReflect.Descriptor instead.
func (*FileBlob) Descriptor() ([]byte, []int) {
	return file_Blobs_proto_rawDescGZIP(), []int{2}
}

func (x *FileBlob) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *FileBlob) GetData() *Blob {
	if x != nil {
		return x.Data
	}
	return nil
}

type Blob_CompressionInfo struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Method       Blob_CompressionInfo_CompressionType `protobuf:"varint,1,opt,name=method,proto3,enum=contester.proto.Blob_CompressionInfo_CompressionType" json:"method,omitempty"`
	OriginalSize uint32                               `protobuf:"varint,2,opt,name=original_size,json=originalSize,proto3" json:"original_size,omitempty"`
}

func (x *Blob_CompressionInfo) Reset() {
	*x = Blob_CompressionInfo{}
	if protoimpl.UnsafeEnabled {
		mi := &file_Blobs_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Blob_CompressionInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Blob_CompressionInfo) ProtoMessage() {}

func (x *Blob_CompressionInfo) ProtoReflect() protoreflect.Message {
	mi := &file_Blobs_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Blob_CompressionInfo.ProtoReflect.Descriptor instead.
func (*Blob_CompressionInfo) Descriptor() ([]byte, []int) {
	return file_Blobs_proto_rawDescGZIP(), []int{0, 0}
}

func (x *Blob_CompressionInfo) GetMethod() Blob_CompressionInfo_CompressionType {
	if x != nil {
		return x.Method
	}
	return Blob_CompressionInfo_METHOD_NONE
}

func (x *Blob_CompressionInfo) GetOriginalSize() uint32 {
	if x != nil {
		return x.OriginalSize
	}
	return 0
}

var File_Blobs_proto protoreflect.FileDescriptor

var file_Blobs_proto_rawDesc = []byte{
	0x0a, 0x0b, 0x42, 0x6c, 0x6f, 0x62, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0f, 0x63,
	0x6f, 0x6e, 0x74, 0x65, 0x73, 0x74, 0x65, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xb4,
	0x02, 0x0a, 0x04, 0x42, 0x6c, 0x6f, 0x62, 0x12, 0x12, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x12, 0x47, 0x0a, 0x0b, 0x63,
	0x6f, 0x6d, 0x70, 0x72, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x25, 0x2e, 0x63, 0x6f, 0x6e, 0x74, 0x65, 0x73, 0x74, 0x65, 0x72, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x2e, 0x42, 0x6c, 0x6f, 0x62, 0x2e, 0x43, 0x6f, 0x6d, 0x70, 0x72, 0x65, 0x73, 0x73,
	0x69, 0x6f, 0x6e, 0x49, 0x6e, 0x66, 0x6f, 0x52, 0x0b, 0x63, 0x6f, 0x6d, 0x70, 0x72, 0x65, 0x73,
	0x73, 0x69, 0x6f, 0x6e, 0x12, 0x12, 0x0a, 0x04, 0x73, 0x68, 0x61, 0x31, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x0c, 0x52, 0x04, 0x73, 0x68, 0x61, 0x31, 0x1a, 0xba, 0x01, 0x0a, 0x0f, 0x43, 0x6f, 0x6d,
	0x70, 0x72, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x49, 0x6e, 0x66, 0x6f, 0x12, 0x4d, 0x0a, 0x06,
	0x6d, 0x65, 0x74, 0x68, 0x6f, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x35, 0x2e, 0x63,
	0x6f, 0x6e, 0x74, 0x65, 0x73, 0x74, 0x65, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x42,
	0x6c, 0x6f, 0x62, 0x2e, 0x43, 0x6f, 0x6d, 0x70, 0x72, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x49,
	0x6e, 0x66, 0x6f, 0x2e, 0x43, 0x6f, 0x6d, 0x70, 0x72, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x54,
	0x79, 0x70, 0x65, 0x52, 0x06, 0x6d, 0x65, 0x74, 0x68, 0x6f, 0x64, 0x12, 0x23, 0x0a, 0x0d, 0x6f,
	0x72, 0x69, 0x67, 0x69, 0x6e, 0x61, 0x6c, 0x5f, 0x73, 0x69, 0x7a, 0x65, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x0d, 0x52, 0x0c, 0x6f, 0x72, 0x69, 0x67, 0x69, 0x6e, 0x61, 0x6c, 0x53, 0x69, 0x7a, 0x65,
	0x22, 0x33, 0x0a, 0x0f, 0x43, 0x6f, 0x6d, 0x70, 0x72, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x54,
	0x79, 0x70, 0x65, 0x12, 0x0f, 0x0a, 0x0b, 0x4d, 0x45, 0x54, 0x48, 0x4f, 0x44, 0x5f, 0x4e, 0x4f,
	0x4e, 0x45, 0x10, 0x00, 0x12, 0x0f, 0x0a, 0x0b, 0x4d, 0x45, 0x54, 0x48, 0x4f, 0x44, 0x5f, 0x5a,
	0x4c, 0x49, 0x42, 0x10, 0x01, 0x22, 0x5b, 0x0a, 0x06, 0x4d, 0x6f, 0x64, 0x75, 0x6c, 0x65, 0x12,
	0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e,
	0x61, 0x6d, 0x65, 0x12, 0x29, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x15, 0x2e, 0x63, 0x6f, 0x6e, 0x74, 0x65, 0x73, 0x74, 0x65, 0x72, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x2e, 0x42, 0x6c, 0x6f, 0x62, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x12, 0x12,
	0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x74, 0x79,
	0x70, 0x65, 0x22, 0x49, 0x0a, 0x08, 0x46, 0x69, 0x6c, 0x65, 0x42, 0x6c, 0x6f, 0x62, 0x12, 0x12,
	0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61,
	0x6d, 0x65, 0x12, 0x29, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x15, 0x2e, 0x63, 0x6f, 0x6e, 0x74, 0x65, 0x73, 0x74, 0x65, 0x72, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x2e, 0x42, 0x6c, 0x6f, 0x62, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x42, 0x4b, 0x0a,
	0x1c, 0x6f, 0x72, 0x67, 0x2e, 0x73, 0x74, 0x69, 0x6e, 0x67, 0x72, 0x61, 0x79, 0x2e, 0x63, 0x6f,
	0x6e, 0x74, 0x65, 0x73, 0x74, 0x65, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x5a, 0x2b, 0x67,
	0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x63, 0x6f, 0x6e, 0x74, 0x65, 0x73,
	0x74, 0x65, 0x72, 0x2f, 0x72, 0x75, 0x6e, 0x6c, 0x69, 0x62, 0x2f, 0x63, 0x6f, 0x6e, 0x74, 0x65,
	0x73, 0x74, 0x65, 0x72, 0x5f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x33,
}

var (
	file_Blobs_proto_rawDescOnce sync.Once
	file_Blobs_proto_rawDescData = file_Blobs_proto_rawDesc
)

func file_Blobs_proto_rawDescGZIP() []byte {
	file_Blobs_proto_rawDescOnce.Do(func() {
		file_Blobs_proto_rawDescData = protoimpl.X.CompressGZIP(file_Blobs_proto_rawDescData)
	})
	return file_Blobs_proto_rawDescData
}

var file_Blobs_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_Blobs_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_Blobs_proto_goTypes = []interface{}{
	(Blob_CompressionInfo_CompressionType)(0), // 0: contester.proto.Blob.CompressionInfo.CompressionType
	(*Blob)(nil),                 // 1: contester.proto.Blob
	(*Module)(nil),               // 2: contester.proto.Module
	(*FileBlob)(nil),             // 3: contester.proto.FileBlob
	(*Blob_CompressionInfo)(nil), // 4: contester.proto.Blob.CompressionInfo
}
var file_Blobs_proto_depIdxs = []int32{
	4, // 0: contester.proto.Blob.compression:type_name -> contester.proto.Blob.CompressionInfo
	1, // 1: contester.proto.Module.data:type_name -> contester.proto.Blob
	1, // 2: contester.proto.FileBlob.data:type_name -> contester.proto.Blob
	0, // 3: contester.proto.Blob.CompressionInfo.method:type_name -> contester.proto.Blob.CompressionInfo.CompressionType
	4, // [4:4] is the sub-list for method output_type
	4, // [4:4] is the sub-list for method input_type
	4, // [4:4] is the sub-list for extension type_name
	4, // [4:4] is the sub-list for extension extendee
	0, // [0:4] is the sub-list for field type_name
}

func init() { file_Blobs_proto_init() }
func file_Blobs_proto_init() {
	if File_Blobs_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_Blobs_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Blob); i {
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
		file_Blobs_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Module); i {
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
		file_Blobs_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*FileBlob); i {
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
		file_Blobs_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Blob_CompressionInfo); i {
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
			RawDescriptor: file_Blobs_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_Blobs_proto_goTypes,
		DependencyIndexes: file_Blobs_proto_depIdxs,
		EnumInfos:         file_Blobs_proto_enumTypes,
		MessageInfos:      file_Blobs_proto_msgTypes,
	}.Build()
	File_Blobs_proto = out.File
	file_Blobs_proto_rawDesc = nil
	file_Blobs_proto_goTypes = nil
	file_Blobs_proto_depIdxs = nil
}
