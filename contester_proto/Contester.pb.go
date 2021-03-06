// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: Contester.proto

package contester_proto

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

import io "io"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type Compilation_Code int32

const (
	Compilation_Unknown Compilation_Code = 0
	Compilation_Success Compilation_Code = 1
	Compilation_Failure Compilation_Code = 2
)

var Compilation_Code_name = map[int32]string{
	0: "Unknown",
	1: "Success",
	2: "Failure",
}
var Compilation_Code_value = map[string]int32{
	"Unknown": 0,
	"Success": 1,
	"Failure": 2,
}

func (x Compilation_Code) String() string {
	return proto.EnumName(Compilation_Code_name, int32(x))
}
func (Compilation_Code) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_Contester_e260d07f24f32b1e, []int{0, 0}
}

type Compilation struct {
	Failure              bool                  `protobuf:"varint,1,opt,name=failure,proto3" json:"failure,omitempty"`
	ResultSteps          []*Compilation_Result `protobuf:"bytes,2,rep,name=result_steps,json=resultSteps,proto3" json:"result_steps,omitempty"`
	XXX_NoUnkeyedLiteral struct{}              `json:"-"`
	XXX_unrecognized     []byte                `json:"-"`
	XXX_sizecache        int32                 `json:"-"`
}

func (m *Compilation) Reset()         { *m = Compilation{} }
func (m *Compilation) String() string { return proto.CompactTextString(m) }
func (*Compilation) ProtoMessage()    {}
func (*Compilation) Descriptor() ([]byte, []int) {
	return fileDescriptor_Contester_e260d07f24f32b1e, []int{0}
}
func (m *Compilation) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Compilation) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Compilation.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalTo(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (dst *Compilation) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Compilation.Merge(dst, src)
}
func (m *Compilation) XXX_Size() int {
	return m.Size()
}
func (m *Compilation) XXX_DiscardUnknown() {
	xxx_messageInfo_Compilation.DiscardUnknown(m)
}

var xxx_messageInfo_Compilation proto.InternalMessageInfo

func (m *Compilation) GetFailure() bool {
	if m != nil {
		return m.Failure
	}
	return false
}

func (m *Compilation) GetResultSteps() []*Compilation_Result {
	if m != nil {
		return m.ResultSteps
	}
	return nil
}

type Compilation_Result struct {
	StepName             string          `protobuf:"bytes,1,opt,name=step_name,json=stepName,proto3" json:"step_name,omitempty"`
	Execution            *LocalExecution `protobuf:"bytes,2,opt,name=execution,proto3" json:"execution,omitempty"`
	Failure              bool            `protobuf:"varint,3,opt,name=failure,proto3" json:"failure,omitempty"`
	XXX_NoUnkeyedLiteral struct{}        `json:"-"`
	XXX_unrecognized     []byte          `json:"-"`
	XXX_sizecache        int32           `json:"-"`
}

func (m *Compilation_Result) Reset()         { *m = Compilation_Result{} }
func (m *Compilation_Result) String() string { return proto.CompactTextString(m) }
func (*Compilation_Result) ProtoMessage()    {}
func (*Compilation_Result) Descriptor() ([]byte, []int) {
	return fileDescriptor_Contester_e260d07f24f32b1e, []int{0, 0}
}
func (m *Compilation_Result) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Compilation_Result) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Compilation_Result.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalTo(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (dst *Compilation_Result) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Compilation_Result.Merge(dst, src)
}
func (m *Compilation_Result) XXX_Size() int {
	return m.Size()
}
func (m *Compilation_Result) XXX_DiscardUnknown() {
	xxx_messageInfo_Compilation_Result.DiscardUnknown(m)
}

var xxx_messageInfo_Compilation_Result proto.InternalMessageInfo

func (m *Compilation_Result) GetStepName() string {
	if m != nil {
		return m.StepName
	}
	return ""
}

func (m *Compilation_Result) GetExecution() *LocalExecution {
	if m != nil {
		return m.Execution
	}
	return nil
}

func (m *Compilation_Result) GetFailure() bool {
	if m != nil {
		return m.Failure
	}
	return false
}

func init() {
	proto.RegisterType((*Compilation)(nil), "contester.proto.Compilation")
	proto.RegisterType((*Compilation_Result)(nil), "contester.proto.Compilation.Result")
	proto.RegisterEnum("contester.proto.Compilation_Code", Compilation_Code_name, Compilation_Code_value)
}
func (m *Compilation) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Compilation) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if m.Failure {
		dAtA[i] = 0x8
		i++
		if m.Failure {
			dAtA[i] = 1
		} else {
			dAtA[i] = 0
		}
		i++
	}
	if len(m.ResultSteps) > 0 {
		for _, msg := range m.ResultSteps {
			dAtA[i] = 0x12
			i++
			i = encodeVarintContester(dAtA, i, uint64(msg.Size()))
			n, err := msg.MarshalTo(dAtA[i:])
			if err != nil {
				return 0, err
			}
			i += n
		}
	}
	if m.XXX_unrecognized != nil {
		i += copy(dAtA[i:], m.XXX_unrecognized)
	}
	return i, nil
}

func (m *Compilation_Result) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Compilation_Result) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if len(m.StepName) > 0 {
		dAtA[i] = 0xa
		i++
		i = encodeVarintContester(dAtA, i, uint64(len(m.StepName)))
		i += copy(dAtA[i:], m.StepName)
	}
	if m.Execution != nil {
		dAtA[i] = 0x12
		i++
		i = encodeVarintContester(dAtA, i, uint64(m.Execution.Size()))
		n1, err := m.Execution.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n1
	}
	if m.Failure {
		dAtA[i] = 0x18
		i++
		if m.Failure {
			dAtA[i] = 1
		} else {
			dAtA[i] = 0
		}
		i++
	}
	if m.XXX_unrecognized != nil {
		i += copy(dAtA[i:], m.XXX_unrecognized)
	}
	return i, nil
}

func encodeVarintContester(dAtA []byte, offset int, v uint64) int {
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return offset + 1
}
func (m *Compilation) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Failure {
		n += 2
	}
	if len(m.ResultSteps) > 0 {
		for _, e := range m.ResultSteps {
			l = e.Size()
			n += 1 + l + sovContester(uint64(l))
		}
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func (m *Compilation_Result) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.StepName)
	if l > 0 {
		n += 1 + l + sovContester(uint64(l))
	}
	if m.Execution != nil {
		l = m.Execution.Size()
		n += 1 + l + sovContester(uint64(l))
	}
	if m.Failure {
		n += 2
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func sovContester(x uint64) (n int) {
	for {
		n++
		x >>= 7
		if x == 0 {
			break
		}
	}
	return n
}
func sozContester(x uint64) (n int) {
	return sovContester(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *Compilation) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowContester
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Compilation: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Compilation: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Failure", wireType)
			}
			var v int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowContester
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				v |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			m.Failure = bool(v != 0)
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field ResultSteps", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowContester
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthContester
			}
			postIndex := iNdEx + msglen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.ResultSteps = append(m.ResultSteps, &Compilation_Result{})
			if err := m.ResultSteps[len(m.ResultSteps)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipContester(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthContester
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *Compilation_Result) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowContester
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Result: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Result: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field StepName", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowContester
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthContester
			}
			postIndex := iNdEx + intStringLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.StepName = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Execution", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowContester
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthContester
			}
			postIndex := iNdEx + msglen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Execution == nil {
				m.Execution = &LocalExecution{}
			}
			if err := m.Execution.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 3:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Failure", wireType)
			}
			var v int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowContester
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				v |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			m.Failure = bool(v != 0)
		default:
			iNdEx = preIndex
			skippy, err := skipContester(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthContester
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipContester(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowContester
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowContester
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
			return iNdEx, nil
		case 1:
			iNdEx += 8
			return iNdEx, nil
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowContester
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			iNdEx += length
			if length < 0 {
				return 0, ErrInvalidLengthContester
			}
			return iNdEx, nil
		case 3:
			for {
				var innerWire uint64
				var start int = iNdEx
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return 0, ErrIntOverflowContester
					}
					if iNdEx >= l {
						return 0, io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					innerWire |= (uint64(b) & 0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				innerWireType := int(innerWire & 0x7)
				if innerWireType == 4 {
					break
				}
				next, err := skipContester(dAtA[start:])
				if err != nil {
					return 0, err
				}
				iNdEx = start + next
			}
			return iNdEx, nil
		case 4:
			return iNdEx, nil
		case 5:
			iNdEx += 4
			return iNdEx, nil
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
	}
	panic("unreachable")
}

var (
	ErrInvalidLengthContester = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowContester   = fmt.Errorf("proto: integer overflow")
)

func init() { proto.RegisterFile("Contester.proto", fileDescriptor_Contester_e260d07f24f32b1e) }

var fileDescriptor_Contester_e260d07f24f32b1e = []byte{
	// 273 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0x77, 0xce, 0xcf, 0x2b,
	0x49, 0x2d, 0x2e, 0x49, 0x2d, 0xd2, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0xe2, 0x4f, 0x46, 0x15,
	0x90, 0xe2, 0xf6, 0xc9, 0x4f, 0x4e, 0xcc, 0x81, 0x70, 0x94, 0xe6, 0x31, 0x71, 0x71, 0x3b, 0xe7,
	0xe7, 0x16, 0x64, 0xe6, 0x24, 0x96, 0x64, 0xe6, 0xe7, 0x09, 0x49, 0x70, 0xb1, 0xa7, 0x25, 0x66,
	0xe6, 0x94, 0x16, 0xa5, 0x4a, 0x30, 0x2a, 0x30, 0x6a, 0x70, 0x04, 0xc1, 0xb8, 0x42, 0x6e, 0x5c,
	0x3c, 0x45, 0xa9, 0xc5, 0xa5, 0x39, 0x25, 0xf1, 0xc5, 0x25, 0xa9, 0x05, 0xc5, 0x12, 0x4c, 0x0a,
	0xcc, 0x1a, 0xdc, 0x46, 0xca, 0x7a, 0x68, 0xc6, 0xeb, 0x21, 0x99, 0xa6, 0x17, 0x04, 0xd6, 0x10,
	0xc4, 0x0d, 0xd1, 0x18, 0x0c, 0xd2, 0x27, 0x55, 0xc7, 0xc5, 0x06, 0x11, 0x16, 0x92, 0xe6, 0xe2,
	0x04, 0x19, 0x15, 0x9f, 0x97, 0x98, 0x0b, 0xb1, 0x8d, 0x33, 0x88, 0x03, 0x24, 0xe0, 0x97, 0x98,
	0x9b, 0x2a, 0x64, 0xcb, 0xc5, 0x99, 0x5a, 0x91, 0x9a, 0x5c, 0x0a, 0x32, 0x47, 0x82, 0x49, 0x81,
	0x51, 0x83, 0xdb, 0x48, 0x1e, 0xc3, 0x2e, 0xb0, 0x4f, 0x5c, 0x61, 0xca, 0x82, 0x10, 0x3a, 0x90,
	0xfd, 0xc1, 0x8c, 0xe2, 0x0f, 0x25, 0x5d, 0x2e, 0x16, 0xe7, 0xfc, 0x94, 0x54, 0x21, 0x6e, 0x2e,
	0xf6, 0xd0, 0xbc, 0xec, 0xbc, 0xfc, 0xf2, 0x3c, 0x01, 0x06, 0x10, 0x27, 0xb8, 0x34, 0x39, 0x39,
	0xb5, 0xb8, 0x58, 0x80, 0x11, 0xc4, 0x71, 0x83, 0x28, 0x16, 0x60, 0x72, 0xd2, 0x3b, 0xf1, 0x48,
	0x8e, 0xf1, 0xc2, 0x23, 0x39, 0xc6, 0x07, 0x8f, 0xe4, 0x18, 0x67, 0x3c, 0x96, 0x63, 0xe0, 0x92,
	0xc9, 0x2f, 0x4a, 0xd7, 0x2b, 0x2e, 0xc9, 0xcc, 0x4b, 0x2f, 0x4a, 0xac, 0x44, 0x77, 0x52, 0x12,
	0x1b, 0x98, 0x32, 0x06, 0x04, 0x00, 0x00, 0xff, 0xff, 0x0b, 0xa2, 0x7f, 0xb5, 0x88, 0x01, 0x00,
	0x00,
}
