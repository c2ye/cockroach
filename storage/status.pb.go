// Code generated by protoc-gen-gogo.
// source: cockroach/storage/status.proto
// DO NOT EDIT!

/*
	Package storage is a generated protocol buffer package.

	It is generated from these files:
		cockroach/storage/status.proto

	It has these top-level messages:
		StoreStatus
*/
package storage

import proto "github.com/gogo/protobuf/proto"
import fmt "fmt"
import math "math"
import cockroach_proto "github.com/cockroachdb/cockroach/proto"
import cockroach_storage_engine "github.com/cockroachdb/cockroach/storage/engine"

// discarding unused import gogoproto "github.com/cockroachdb/gogoproto"

import github_com_cockroachdb_cockroach_proto "github.com/cockroachdb/cockroach/proto"

import io "io"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// StoreStatus contains the stats needed to calculate the current status of a
// store.
type StoreStatus struct {
	Desc                 cockroach_proto.StoreDescriptor               `protobuf:"bytes,1,opt,name=desc" json:"desc"`
	NodeID               github_com_cockroachdb_cockroach_proto.NodeID `protobuf:"varint,2,opt,name=node_id,casttype=github.com/cockroachdb/cockroach/proto.NodeID" json:"node_id"`
	RangeCount           int32                                         `protobuf:"varint,3,opt,name=range_count" json:"range_count"`
	StartedAt            int64                                         `protobuf:"varint,4,opt,name=started_at" json:"started_at"`
	UpdatedAt            int64                                         `protobuf:"varint,5,opt,name=updated_at" json:"updated_at"`
	Stats                cockroach_storage_engine.MVCCStats            `protobuf:"bytes,6,opt,name=stats" json:"stats"`
	LeaderRangeCount     int32                                         `protobuf:"varint,7,opt,name=leader_range_count" json:"leader_range_count"`
	ReplicatedRangeCount int32                                         `protobuf:"varint,8,opt,name=replicated_range_count" json:"replicated_range_count"`
	AvailableRangeCount  int32                                         `protobuf:"varint,9,opt,name=available_range_count" json:"available_range_count"`
}

func (m *StoreStatus) Reset()         { *m = StoreStatus{} }
func (m *StoreStatus) String() string { return proto.CompactTextString(m) }
func (*StoreStatus) ProtoMessage()    {}

func (m *StoreStatus) GetDesc() cockroach_proto.StoreDescriptor {
	if m != nil {
		return m.Desc
	}
	return cockroach_proto.StoreDescriptor{}
}

func (m *StoreStatus) GetNodeID() github_com_cockroachdb_cockroach_proto.NodeID {
	if m != nil {
		return m.NodeID
	}
	return 0
}

func (m *StoreStatus) GetRangeCount() int32 {
	if m != nil {
		return m.RangeCount
	}
	return 0
}

func (m *StoreStatus) GetStartedAt() int64 {
	if m != nil {
		return m.StartedAt
	}
	return 0
}

func (m *StoreStatus) GetUpdatedAt() int64 {
	if m != nil {
		return m.UpdatedAt
	}
	return 0
}

func (m *StoreStatus) GetStats() cockroach_storage_engine.MVCCStats {
	if m != nil {
		return m.Stats
	}
	return cockroach_storage_engine.MVCCStats{}
}

func (m *StoreStatus) GetLeaderRangeCount() int32 {
	if m != nil {
		return m.LeaderRangeCount
	}
	return 0
}

func (m *StoreStatus) GetReplicatedRangeCount() int32 {
	if m != nil {
		return m.ReplicatedRangeCount
	}
	return 0
}

func (m *StoreStatus) GetAvailableRangeCount() int32 {
	if m != nil {
		return m.AvailableRangeCount
	}
	return 0
}

func (m *StoreStatus) Marshal() (data []byte, err error) {
	size := m.Size()
	data = make([]byte, size)
	n, err := m.MarshalTo(data)
	if err != nil {
		return nil, err
	}
	return data[:n], nil
}

func (m *StoreStatus) MarshalTo(data []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	data[i] = 0xa
	i++
	i = encodeVarintStatus(data, i, uint64(m.Desc.Size()))
	n1, err := m.Desc.MarshalTo(data[i:])
	if err != nil {
		return 0, err
	}
	i += n1
	data[i] = 0x10
	i++
	i = encodeVarintStatus(data, i, uint64(m.NodeID))
	data[i] = 0x18
	i++
	i = encodeVarintStatus(data, i, uint64(m.RangeCount))
	data[i] = 0x20
	i++
	i = encodeVarintStatus(data, i, uint64(m.StartedAt))
	data[i] = 0x28
	i++
	i = encodeVarintStatus(data, i, uint64(m.UpdatedAt))
	data[i] = 0x32
	i++
	i = encodeVarintStatus(data, i, uint64(m.Stats.Size()))
	n2, err := m.Stats.MarshalTo(data[i:])
	if err != nil {
		return 0, err
	}
	i += n2
	data[i] = 0x38
	i++
	i = encodeVarintStatus(data, i, uint64(m.LeaderRangeCount))
	data[i] = 0x40
	i++
	i = encodeVarintStatus(data, i, uint64(m.ReplicatedRangeCount))
	data[i] = 0x48
	i++
	i = encodeVarintStatus(data, i, uint64(m.AvailableRangeCount))
	return i, nil
}

func encodeFixed64Status(data []byte, offset int, v uint64) int {
	data[offset] = uint8(v)
	data[offset+1] = uint8(v >> 8)
	data[offset+2] = uint8(v >> 16)
	data[offset+3] = uint8(v >> 24)
	data[offset+4] = uint8(v >> 32)
	data[offset+5] = uint8(v >> 40)
	data[offset+6] = uint8(v >> 48)
	data[offset+7] = uint8(v >> 56)
	return offset + 8
}
func encodeFixed32Status(data []byte, offset int, v uint32) int {
	data[offset] = uint8(v)
	data[offset+1] = uint8(v >> 8)
	data[offset+2] = uint8(v >> 16)
	data[offset+3] = uint8(v >> 24)
	return offset + 4
}
func encodeVarintStatus(data []byte, offset int, v uint64) int {
	for v >= 1<<7 {
		data[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	data[offset] = uint8(v)
	return offset + 1
}
func (m *StoreStatus) Size() (n int) {
	var l int
	_ = l
	l = m.Desc.Size()
	n += 1 + l + sovStatus(uint64(l))
	n += 1 + sovStatus(uint64(m.NodeID))
	n += 1 + sovStatus(uint64(m.RangeCount))
	n += 1 + sovStatus(uint64(m.StartedAt))
	n += 1 + sovStatus(uint64(m.UpdatedAt))
	l = m.Stats.Size()
	n += 1 + l + sovStatus(uint64(l))
	n += 1 + sovStatus(uint64(m.LeaderRangeCount))
	n += 1 + sovStatus(uint64(m.ReplicatedRangeCount))
	n += 1 + sovStatus(uint64(m.AvailableRangeCount))
	return n
}

func sovStatus(x uint64) (n int) {
	for {
		n++
		x >>= 7
		if x == 0 {
			break
		}
	}
	return n
}
func sozStatus(x uint64) (n int) {
	return sovStatus(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *StoreStatus) Unmarshal(data []byte) error {
	l := len(data)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowStatus
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := data[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: StoreStatus: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: StoreStatus: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Desc", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowStatus
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[iNdEx]
				iNdEx++
				msglen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthStatus
			}
			postIndex := iNdEx + msglen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := m.Desc.Unmarshal(data[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field NodeID", wireType)
			}
			m.NodeID = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowStatus
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[iNdEx]
				iNdEx++
				m.NodeID |= (github_com_cockroachdb_cockroach_proto.NodeID(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 3:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field RangeCount", wireType)
			}
			m.RangeCount = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowStatus
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[iNdEx]
				iNdEx++
				m.RangeCount |= (int32(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 4:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field StartedAt", wireType)
			}
			m.StartedAt = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowStatus
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[iNdEx]
				iNdEx++
				m.StartedAt |= (int64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 5:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field UpdatedAt", wireType)
			}
			m.UpdatedAt = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowStatus
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[iNdEx]
				iNdEx++
				m.UpdatedAt |= (int64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 6:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Stats", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowStatus
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[iNdEx]
				iNdEx++
				msglen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthStatus
			}
			postIndex := iNdEx + msglen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := m.Stats.Unmarshal(data[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 7:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field LeaderRangeCount", wireType)
			}
			m.LeaderRangeCount = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowStatus
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[iNdEx]
				iNdEx++
				m.LeaderRangeCount |= (int32(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 8:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field ReplicatedRangeCount", wireType)
			}
			m.ReplicatedRangeCount = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowStatus
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[iNdEx]
				iNdEx++
				m.ReplicatedRangeCount |= (int32(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 9:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field AvailableRangeCount", wireType)
			}
			m.AvailableRangeCount = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowStatus
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := data[iNdEx]
				iNdEx++
				m.AvailableRangeCount |= (int32(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipStatus(data[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthStatus
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipStatus(data []byte) (n int, err error) {
	l := len(data)
	iNdEx := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowStatus
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := data[iNdEx]
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
					return 0, ErrIntOverflowStatus
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if data[iNdEx-1] < 0x80 {
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
					return 0, ErrIntOverflowStatus
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := data[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			iNdEx += length
			if length < 0 {
				return 0, ErrInvalidLengthStatus
			}
			return iNdEx, nil
		case 3:
			for {
				var innerWire uint64
				var start int = iNdEx
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return 0, ErrIntOverflowStatus
					}
					if iNdEx >= l {
						return 0, io.ErrUnexpectedEOF
					}
					b := data[iNdEx]
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
				next, err := skipStatus(data[start:])
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
	ErrInvalidLengthStatus = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowStatus   = fmt.Errorf("proto: integer overflow")
)
