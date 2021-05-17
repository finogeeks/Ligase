// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.26.0
// 	protoc        v3.15.8
// source: syncaggregate.proto

package pb

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
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

type UpdateOneTimeKeyReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	UserID   string `protobuf:"bytes,1,opt,name=userID,proto3" json:"userID,omitempty"`
	DeviceID string `protobuf:"bytes,2,opt,name=deviceID,proto3" json:"deviceID,omitempty"`
}

func (x *UpdateOneTimeKeyReq) Reset() {
	*x = UpdateOneTimeKeyReq{}
	if protoimpl.UnsafeEnabled {
		mi := &file_syncaggregate_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UpdateOneTimeKeyReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdateOneTimeKeyReq) ProtoMessage() {}

func (x *UpdateOneTimeKeyReq) ProtoReflect() protoreflect.Message {
	mi := &file_syncaggregate_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UpdateOneTimeKeyReq.ProtoReflect.Descriptor instead.
func (*UpdateOneTimeKeyReq) Descriptor() ([]byte, []int) {
	return file_syncaggregate_proto_rawDescGZIP(), []int{0}
}

func (x *UpdateOneTimeKeyReq) GetUserID() string {
	if x != nil {
		return x.UserID
	}
	return ""
}

func (x *UpdateOneTimeKeyReq) GetDeviceID() string {
	if x != nil {
		return x.DeviceID
	}
	return ""
}

type DeviceKeyChanges struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Offset        int64  `protobuf:"varint,1,opt,name=offset,proto3" json:"offset,omitempty"`
	ChangedUserID string `protobuf:"bytes,2,opt,name=changedUserID,proto3" json:"changedUserID,omitempty"`
}

func (x *DeviceKeyChanges) Reset() {
	*x = DeviceKeyChanges{}
	if protoimpl.UnsafeEnabled {
		mi := &file_syncaggregate_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DeviceKeyChanges) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DeviceKeyChanges) ProtoMessage() {}

func (x *DeviceKeyChanges) ProtoReflect() protoreflect.Message {
	mi := &file_syncaggregate_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DeviceKeyChanges.ProtoReflect.Descriptor instead.
func (*DeviceKeyChanges) Descriptor() ([]byte, []int) {
	return file_syncaggregate_proto_rawDescGZIP(), []int{1}
}

func (x *DeviceKeyChanges) GetOffset() int64 {
	if x != nil {
		return x.Offset
	}
	return 0
}

func (x *DeviceKeyChanges) GetChangedUserID() string {
	if x != nil {
		return x.ChangedUserID
	}
	return ""
}

type UpdateDeviceKeyReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	DeviceKeyChanges []*DeviceKeyChanges `protobuf:"bytes,1,rep,name=deviceKeyChanges,proto3" json:"deviceKeyChanges,omitempty"`
	EventNID         int64               `protobuf:"varint,2,opt,name=eventNID,proto3" json:"eventNID,omitempty"`
}

func (x *UpdateDeviceKeyReq) Reset() {
	*x = UpdateDeviceKeyReq{}
	if protoimpl.UnsafeEnabled {
		mi := &file_syncaggregate_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UpdateDeviceKeyReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdateDeviceKeyReq) ProtoMessage() {}

func (x *UpdateDeviceKeyReq) ProtoReflect() protoreflect.Message {
	mi := &file_syncaggregate_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UpdateDeviceKeyReq.ProtoReflect.Descriptor instead.
func (*UpdateDeviceKeyReq) Descriptor() ([]byte, []int) {
	return file_syncaggregate_proto_rawDescGZIP(), []int{2}
}

func (x *UpdateDeviceKeyReq) GetDeviceKeyChanges() []*DeviceKeyChanges {
	if x != nil {
		return x.DeviceKeyChanges
	}
	return nil
}

func (x *UpdateDeviceKeyReq) GetEventNID() int64 {
	if x != nil {
		return x.EventNID
	}
	return 0
}

type GetOnlinePresenceReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	UserID string `protobuf:"bytes,1,opt,name=userID,proto3" json:"userID,omitempty"`
}

func (x *GetOnlinePresenceReq) Reset() {
	*x = GetOnlinePresenceReq{}
	if protoimpl.UnsafeEnabled {
		mi := &file_syncaggregate_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetOnlinePresenceReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetOnlinePresenceReq) ProtoMessage() {}

func (x *GetOnlinePresenceReq) ProtoReflect() protoreflect.Message {
	mi := &file_syncaggregate_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetOnlinePresenceReq.ProtoReflect.Descriptor instead.
func (*GetOnlinePresenceReq) Descriptor() ([]byte, []int) {
	return file_syncaggregate_proto_rawDescGZIP(), []int{3}
}

func (x *GetOnlinePresenceReq) GetUserID() string {
	if x != nil {
		return x.UserID
	}
	return ""
}

type GetOnlinePresenceRsp struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Found        bool   `protobuf:"varint,1,opt,name=found,proto3" json:"found,omitempty"`
	Presence     string `protobuf:"bytes,2,opt,name=presence,proto3" json:"presence,omitempty"`
	StatusMsg    string `protobuf:"bytes,3,opt,name=statusMsg,proto3" json:"statusMsg,omitempty"`
	ExtStatusMsg string `protobuf:"bytes,4,opt,name=extStatusMsg,proto3" json:"extStatusMsg,omitempty"`
}

func (x *GetOnlinePresenceRsp) Reset() {
	*x = GetOnlinePresenceRsp{}
	if protoimpl.UnsafeEnabled {
		mi := &file_syncaggregate_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetOnlinePresenceRsp) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetOnlinePresenceRsp) ProtoMessage() {}

func (x *GetOnlinePresenceRsp) ProtoReflect() protoreflect.Message {
	mi := &file_syncaggregate_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetOnlinePresenceRsp.ProtoReflect.Descriptor instead.
func (*GetOnlinePresenceRsp) Descriptor() ([]byte, []int) {
	return file_syncaggregate_proto_rawDescGZIP(), []int{4}
}

func (x *GetOnlinePresenceRsp) GetFound() bool {
	if x != nil {
		return x.Found
	}
	return false
}

func (x *GetOnlinePresenceRsp) GetPresence() string {
	if x != nil {
		return x.Presence
	}
	return ""
}

func (x *GetOnlinePresenceRsp) GetStatusMsg() string {
	if x != nil {
		return x.StatusMsg
	}
	return ""
}

func (x *GetOnlinePresenceRsp) GetExtStatusMsg() string {
	if x != nil {
		return x.ExtStatusMsg
	}
	return ""
}

type SetReceiptLatestReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Users  []string `protobuf:"bytes,1,rep,name=users,proto3" json:"users,omitempty"`
	Offset int64    `protobuf:"varint,2,opt,name=offset,proto3" json:"offset,omitempty"`
	RoomID string   `protobuf:"bytes,3,opt,name=roomID,proto3" json:"roomID,omitempty"`
}

func (x *SetReceiptLatestReq) Reset() {
	*x = SetReceiptLatestReq{}
	if protoimpl.UnsafeEnabled {
		mi := &file_syncaggregate_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SetReceiptLatestReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SetReceiptLatestReq) ProtoMessage() {}

func (x *SetReceiptLatestReq) ProtoReflect() protoreflect.Message {
	mi := &file_syncaggregate_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SetReceiptLatestReq.ProtoReflect.Descriptor instead.
func (*SetReceiptLatestReq) Descriptor() ([]byte, []int) {
	return file_syncaggregate_proto_rawDescGZIP(), []int{5}
}

func (x *SetReceiptLatestReq) GetUsers() []string {
	if x != nil {
		return x.Users
	}
	return nil
}

func (x *SetReceiptLatestReq) GetOffset() int64 {
	if x != nil {
		return x.Offset
	}
	return 0
}

func (x *SetReceiptLatestReq) GetRoomID() string {
	if x != nil {
		return x.RoomID
	}
	return ""
}

type UpdateTypingReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	RoomID    string   `protobuf:"bytes,1,opt,name=roomID,proto3" json:"roomID,omitempty"`
	UserID    string   `protobuf:"bytes,2,opt,name=userID,proto3" json:"userID,omitempty"`
	DeviceID  string   `protobuf:"bytes,3,opt,name=deviceID,proto3" json:"deviceID,omitempty"`
	RoomUsers []string `protobuf:"bytes,4,rep,name=roomUsers,proto3" json:"roomUsers,omitempty"`
}

func (x *UpdateTypingReq) Reset() {
	*x = UpdateTypingReq{}
	if protoimpl.UnsafeEnabled {
		mi := &file_syncaggregate_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UpdateTypingReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdateTypingReq) ProtoMessage() {}

func (x *UpdateTypingReq) ProtoReflect() protoreflect.Message {
	mi := &file_syncaggregate_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UpdateTypingReq.ProtoReflect.Descriptor instead.
func (*UpdateTypingReq) Descriptor() ([]byte, []int) {
	return file_syncaggregate_proto_rawDescGZIP(), []int{6}
}

func (x *UpdateTypingReq) GetRoomID() string {
	if x != nil {
		return x.RoomID
	}
	return ""
}

func (x *UpdateTypingReq) GetUserID() string {
	if x != nil {
		return x.UserID
	}
	return ""
}

func (x *UpdateTypingReq) GetDeviceID() string {
	if x != nil {
		return x.DeviceID
	}
	return ""
}

func (x *UpdateTypingReq) GetRoomUsers() []string {
	if x != nil {
		return x.RoomUsers
	}
	return nil
}

var File_syncaggregate_proto protoreflect.FileDescriptor

var file_syncaggregate_proto_rawDesc = []byte{
	0x0a, 0x13, 0x73, 0x79, 0x6e, 0x63, 0x61, 0x67, 0x67, 0x72, 0x65, 0x67, 0x61, 0x74, 0x65, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x02, 0x70, 0x62, 0x1a, 0x0c, 0x63, 0x6f, 0x6d, 0x6d, 0x6f,
	0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x49, 0x0a, 0x13, 0x55, 0x70, 0x64, 0x61, 0x74,
	0x65, 0x4f, 0x6e, 0x65, 0x54, 0x69, 0x6d, 0x65, 0x4b, 0x65, 0x79, 0x52, 0x65, 0x71, 0x12, 0x16,
	0x0a, 0x06, 0x75, 0x73, 0x65, 0x72, 0x49, 0x44, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06,
	0x75, 0x73, 0x65, 0x72, 0x49, 0x44, 0x12, 0x1a, 0x0a, 0x08, 0x64, 0x65, 0x76, 0x69, 0x63, 0x65,
	0x49, 0x44, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x64, 0x65, 0x76, 0x69, 0x63, 0x65,
	0x49, 0x44, 0x22, 0x50, 0x0a, 0x10, 0x44, 0x65, 0x76, 0x69, 0x63, 0x65, 0x4b, 0x65, 0x79, 0x43,
	0x68, 0x61, 0x6e, 0x67, 0x65, 0x73, 0x12, 0x16, 0x0a, 0x06, 0x6f, 0x66, 0x66, 0x73, 0x65, 0x74,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x06, 0x6f, 0x66, 0x66, 0x73, 0x65, 0x74, 0x12, 0x24,
	0x0a, 0x0d, 0x63, 0x68, 0x61, 0x6e, 0x67, 0x65, 0x64, 0x55, 0x73, 0x65, 0x72, 0x49, 0x44, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0d, 0x63, 0x68, 0x61, 0x6e, 0x67, 0x65, 0x64, 0x55, 0x73,
	0x65, 0x72, 0x49, 0x44, 0x22, 0x72, 0x0a, 0x12, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x44, 0x65,
	0x76, 0x69, 0x63, 0x65, 0x4b, 0x65, 0x79, 0x52, 0x65, 0x71, 0x12, 0x40, 0x0a, 0x10, 0x64, 0x65,
	0x76, 0x69, 0x63, 0x65, 0x4b, 0x65, 0x79, 0x43, 0x68, 0x61, 0x6e, 0x67, 0x65, 0x73, 0x18, 0x01,
	0x20, 0x03, 0x28, 0x0b, 0x32, 0x14, 0x2e, 0x70, 0x62, 0x2e, 0x44, 0x65, 0x76, 0x69, 0x63, 0x65,
	0x4b, 0x65, 0x79, 0x43, 0x68, 0x61, 0x6e, 0x67, 0x65, 0x73, 0x52, 0x10, 0x64, 0x65, 0x76, 0x69,
	0x63, 0x65, 0x4b, 0x65, 0x79, 0x43, 0x68, 0x61, 0x6e, 0x67, 0x65, 0x73, 0x12, 0x1a, 0x0a, 0x08,
	0x65, 0x76, 0x65, 0x6e, 0x74, 0x4e, 0x49, 0x44, 0x18, 0x02, 0x20, 0x01, 0x28, 0x03, 0x52, 0x08,
	0x65, 0x76, 0x65, 0x6e, 0x74, 0x4e, 0x49, 0x44, 0x22, 0x2e, 0x0a, 0x14, 0x47, 0x65, 0x74, 0x4f,
	0x6e, 0x6c, 0x69, 0x6e, 0x65, 0x50, 0x72, 0x65, 0x73, 0x65, 0x6e, 0x63, 0x65, 0x52, 0x65, 0x71,
	0x12, 0x16, 0x0a, 0x06, 0x75, 0x73, 0x65, 0x72, 0x49, 0x44, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x06, 0x75, 0x73, 0x65, 0x72, 0x49, 0x44, 0x22, 0x8a, 0x01, 0x0a, 0x14, 0x47, 0x65, 0x74,
	0x4f, 0x6e, 0x6c, 0x69, 0x6e, 0x65, 0x50, 0x72, 0x65, 0x73, 0x65, 0x6e, 0x63, 0x65, 0x52, 0x73,
	0x70, 0x12, 0x14, 0x0a, 0x05, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x08,
	0x52, 0x05, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x12, 0x1a, 0x0a, 0x08, 0x70, 0x72, 0x65, 0x73, 0x65,
	0x6e, 0x63, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x70, 0x72, 0x65, 0x73, 0x65,
	0x6e, 0x63, 0x65, 0x12, 0x1c, 0x0a, 0x09, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x4d, 0x73, 0x67,
	0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x4d, 0x73,
	0x67, 0x12, 0x22, 0x0a, 0x0c, 0x65, 0x78, 0x74, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x4d, 0x73,
	0x67, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0c, 0x65, 0x78, 0x74, 0x53, 0x74, 0x61, 0x74,
	0x75, 0x73, 0x4d, 0x73, 0x67, 0x22, 0x5b, 0x0a, 0x13, 0x53, 0x65, 0x74, 0x52, 0x65, 0x63, 0x65,
	0x69, 0x70, 0x74, 0x4c, 0x61, 0x74, 0x65, 0x73, 0x74, 0x52, 0x65, 0x71, 0x12, 0x14, 0x0a, 0x05,
	0x75, 0x73, 0x65, 0x72, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x09, 0x52, 0x05, 0x75, 0x73, 0x65,
	0x72, 0x73, 0x12, 0x16, 0x0a, 0x06, 0x6f, 0x66, 0x66, 0x73, 0x65, 0x74, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x03, 0x52, 0x06, 0x6f, 0x66, 0x66, 0x73, 0x65, 0x74, 0x12, 0x16, 0x0a, 0x06, 0x72, 0x6f,
	0x6f, 0x6d, 0x49, 0x44, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x72, 0x6f, 0x6f, 0x6d,
	0x49, 0x44, 0x22, 0x7b, 0x0a, 0x0f, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x54, 0x79, 0x70, 0x69,
	0x6e, 0x67, 0x52, 0x65, 0x71, 0x12, 0x16, 0x0a, 0x06, 0x72, 0x6f, 0x6f, 0x6d, 0x49, 0x44, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x72, 0x6f, 0x6f, 0x6d, 0x49, 0x44, 0x12, 0x16, 0x0a,
	0x06, 0x75, 0x73, 0x65, 0x72, 0x49, 0x44, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x75,
	0x73, 0x65, 0x72, 0x49, 0x44, 0x12, 0x1a, 0x0a, 0x08, 0x64, 0x65, 0x76, 0x69, 0x63, 0x65, 0x49,
	0x44, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x64, 0x65, 0x76, 0x69, 0x63, 0x65, 0x49,
	0x44, 0x12, 0x1c, 0x0a, 0x09, 0x72, 0x6f, 0x6f, 0x6d, 0x55, 0x73, 0x65, 0x72, 0x73, 0x18, 0x04,
	0x20, 0x03, 0x28, 0x09, 0x52, 0x09, 0x72, 0x6f, 0x6f, 0x6d, 0x55, 0x73, 0x65, 0x72, 0x73, 0x32,
	0xe7, 0x02, 0x0a, 0x0d, 0x53, 0x79, 0x6e, 0x63, 0x41, 0x67, 0x67, 0x72, 0x65, 0x67, 0x61, 0x74,
	0x65, 0x12, 0x38, 0x0a, 0x10, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x4f, 0x6e, 0x65, 0x54, 0x69,
	0x6d, 0x65, 0x4b, 0x65, 0x79, 0x12, 0x17, 0x2e, 0x70, 0x62, 0x2e, 0x55, 0x70, 0x64, 0x61, 0x74,
	0x65, 0x4f, 0x6e, 0x65, 0x54, 0x69, 0x6d, 0x65, 0x4b, 0x65, 0x79, 0x52, 0x65, 0x71, 0x1a, 0x09,
	0x2e, 0x70, 0x62, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x22, 0x00, 0x12, 0x36, 0x0a, 0x0f, 0x55,
	0x70, 0x64, 0x61, 0x74, 0x65, 0x44, 0x65, 0x76, 0x69, 0x63, 0x65, 0x4b, 0x65, 0x79, 0x12, 0x16,
	0x2e, 0x70, 0x62, 0x2e, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x44, 0x65, 0x76, 0x69, 0x63, 0x65,
	0x4b, 0x65, 0x79, 0x52, 0x65, 0x71, 0x1a, 0x09, 0x2e, 0x70, 0x62, 0x2e, 0x45, 0x6d, 0x70, 0x74,
	0x79, 0x22, 0x00, 0x12, 0x49, 0x0a, 0x11, 0x47, 0x65, 0x74, 0x4f, 0x6e, 0x6c, 0x69, 0x6e, 0x65,
	0x50, 0x72, 0x65, 0x73, 0x65, 0x6e, 0x63, 0x65, 0x12, 0x18, 0x2e, 0x70, 0x62, 0x2e, 0x47, 0x65,
	0x74, 0x4f, 0x6e, 0x6c, 0x69, 0x6e, 0x65, 0x50, 0x72, 0x65, 0x73, 0x65, 0x6e, 0x63, 0x65, 0x52,
	0x65, 0x71, 0x1a, 0x18, 0x2e, 0x70, 0x62, 0x2e, 0x47, 0x65, 0x74, 0x4f, 0x6e, 0x6c, 0x69, 0x6e,
	0x65, 0x50, 0x72, 0x65, 0x73, 0x65, 0x6e, 0x63, 0x65, 0x52, 0x73, 0x70, 0x22, 0x00, 0x12, 0x38,
	0x0a, 0x10, 0x53, 0x65, 0x74, 0x52, 0x65, 0x63, 0x65, 0x69, 0x70, 0x74, 0x4c, 0x61, 0x74, 0x65,
	0x73, 0x74, 0x12, 0x17, 0x2e, 0x70, 0x62, 0x2e, 0x53, 0x65, 0x74, 0x52, 0x65, 0x63, 0x65, 0x69,
	0x70, 0x74, 0x4c, 0x61, 0x74, 0x65, 0x73, 0x74, 0x52, 0x65, 0x71, 0x1a, 0x09, 0x2e, 0x70, 0x62,
	0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x22, 0x00, 0x12, 0x2d, 0x0a, 0x09, 0x41, 0x64, 0x64, 0x54,
	0x79, 0x70, 0x69, 0x6e, 0x67, 0x12, 0x13, 0x2e, 0x70, 0x62, 0x2e, 0x55, 0x70, 0x64, 0x61, 0x74,
	0x65, 0x54, 0x79, 0x70, 0x69, 0x6e, 0x67, 0x52, 0x65, 0x71, 0x1a, 0x09, 0x2e, 0x70, 0x62, 0x2e,
	0x45, 0x6d, 0x70, 0x74, 0x79, 0x22, 0x00, 0x12, 0x30, 0x0a, 0x0c, 0x52, 0x65, 0x6d, 0x6f, 0x76,
	0x65, 0x54, 0x79, 0x70, 0x69, 0x6e, 0x67, 0x12, 0x13, 0x2e, 0x70, 0x62, 0x2e, 0x55, 0x70, 0x64,
	0x61, 0x74, 0x65, 0x54, 0x79, 0x70, 0x69, 0x6e, 0x67, 0x52, 0x65, 0x71, 0x1a, 0x09, 0x2e, 0x70,
	0x62, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x22, 0x00, 0x42, 0x07, 0x5a, 0x05, 0x2e, 0x2f, 0x3b,
	0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_syncaggregate_proto_rawDescOnce sync.Once
	file_syncaggregate_proto_rawDescData = file_syncaggregate_proto_rawDesc
)

func file_syncaggregate_proto_rawDescGZIP() []byte {
	file_syncaggregate_proto_rawDescOnce.Do(func() {
		file_syncaggregate_proto_rawDescData = protoimpl.X.CompressGZIP(file_syncaggregate_proto_rawDescData)
	})
	return file_syncaggregate_proto_rawDescData
}

var file_syncaggregate_proto_msgTypes = make([]protoimpl.MessageInfo, 7)
var file_syncaggregate_proto_goTypes = []interface{}{
	(*UpdateOneTimeKeyReq)(nil),  // 0: pb.UpdateOneTimeKeyReq
	(*DeviceKeyChanges)(nil),     // 1: pb.DeviceKeyChanges
	(*UpdateDeviceKeyReq)(nil),   // 2: pb.UpdateDeviceKeyReq
	(*GetOnlinePresenceReq)(nil), // 3: pb.GetOnlinePresenceReq
	(*GetOnlinePresenceRsp)(nil), // 4: pb.GetOnlinePresenceRsp
	(*SetReceiptLatestReq)(nil),  // 5: pb.SetReceiptLatestReq
	(*UpdateTypingReq)(nil),      // 6: pb.UpdateTypingReq
	(*Empty)(nil),                // 7: pb.Empty
}
var file_syncaggregate_proto_depIdxs = []int32{
	1, // 0: pb.UpdateDeviceKeyReq.deviceKeyChanges:type_name -> pb.DeviceKeyChanges
	0, // 1: pb.SyncAggregate.UpdateOneTimeKey:input_type -> pb.UpdateOneTimeKeyReq
	2, // 2: pb.SyncAggregate.UpdateDeviceKey:input_type -> pb.UpdateDeviceKeyReq
	3, // 3: pb.SyncAggregate.GetOnlinePresence:input_type -> pb.GetOnlinePresenceReq
	5, // 4: pb.SyncAggregate.SetReceiptLatest:input_type -> pb.SetReceiptLatestReq
	6, // 5: pb.SyncAggregate.AddTyping:input_type -> pb.UpdateTypingReq
	6, // 6: pb.SyncAggregate.RemoveTyping:input_type -> pb.UpdateTypingReq
	7, // 7: pb.SyncAggregate.UpdateOneTimeKey:output_type -> pb.Empty
	7, // 8: pb.SyncAggregate.UpdateDeviceKey:output_type -> pb.Empty
	4, // 9: pb.SyncAggregate.GetOnlinePresence:output_type -> pb.GetOnlinePresenceRsp
	7, // 10: pb.SyncAggregate.SetReceiptLatest:output_type -> pb.Empty
	7, // 11: pb.SyncAggregate.AddTyping:output_type -> pb.Empty
	7, // 12: pb.SyncAggregate.RemoveTyping:output_type -> pb.Empty
	7, // [7:13] is the sub-list for method output_type
	1, // [1:7] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_syncaggregate_proto_init() }
func file_syncaggregate_proto_init() {
	if File_syncaggregate_proto != nil {
		return
	}
	file_common_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_syncaggregate_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UpdateOneTimeKeyReq); i {
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
		file_syncaggregate_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DeviceKeyChanges); i {
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
		file_syncaggregate_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UpdateDeviceKeyReq); i {
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
		file_syncaggregate_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetOnlinePresenceReq); i {
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
		file_syncaggregate_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetOnlinePresenceRsp); i {
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
		file_syncaggregate_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SetReceiptLatestReq); i {
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
		file_syncaggregate_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UpdateTypingReq); i {
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
			RawDescriptor: file_syncaggregate_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   7,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_syncaggregate_proto_goTypes,
		DependencyIndexes: file_syncaggregate_proto_depIdxs,
		MessageInfos:      file_syncaggregate_proto_msgTypes,
	}.Build()
	File_syncaggregate_proto = out.File
	file_syncaggregate_proto_rawDesc = nil
	file_syncaggregate_proto_goTypes = nil
	file_syncaggregate_proto_depIdxs = nil
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConnInterface

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion6

// SyncAggregateClient is the client API for SyncAggregate service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type SyncAggregateClient interface {
	UpdateOneTimeKey(ctx context.Context, in *UpdateOneTimeKeyReq, opts ...grpc.CallOption) (*Empty, error)
	UpdateDeviceKey(ctx context.Context, in *UpdateDeviceKeyReq, opts ...grpc.CallOption) (*Empty, error)
	GetOnlinePresence(ctx context.Context, in *GetOnlinePresenceReq, opts ...grpc.CallOption) (*GetOnlinePresenceRsp, error)
	SetReceiptLatest(ctx context.Context, in *SetReceiptLatestReq, opts ...grpc.CallOption) (*Empty, error)
	AddTyping(ctx context.Context, in *UpdateTypingReq, opts ...grpc.CallOption) (*Empty, error)
	RemoveTyping(ctx context.Context, in *UpdateTypingReq, opts ...grpc.CallOption) (*Empty, error)
}

type syncAggregateClient struct {
	cc grpc.ClientConnInterface
}

func NewSyncAggregateClient(cc grpc.ClientConnInterface) SyncAggregateClient {
	return &syncAggregateClient{cc}
}

func (c *syncAggregateClient) UpdateOneTimeKey(ctx context.Context, in *UpdateOneTimeKeyReq, opts ...grpc.CallOption) (*Empty, error) {
	out := new(Empty)
	err := c.cc.Invoke(ctx, "/pb.SyncAggregate/UpdateOneTimeKey", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *syncAggregateClient) UpdateDeviceKey(ctx context.Context, in *UpdateDeviceKeyReq, opts ...grpc.CallOption) (*Empty, error) {
	out := new(Empty)
	err := c.cc.Invoke(ctx, "/pb.SyncAggregate/UpdateDeviceKey", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *syncAggregateClient) GetOnlinePresence(ctx context.Context, in *GetOnlinePresenceReq, opts ...grpc.CallOption) (*GetOnlinePresenceRsp, error) {
	out := new(GetOnlinePresenceRsp)
	err := c.cc.Invoke(ctx, "/pb.SyncAggregate/GetOnlinePresence", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *syncAggregateClient) SetReceiptLatest(ctx context.Context, in *SetReceiptLatestReq, opts ...grpc.CallOption) (*Empty, error) {
	out := new(Empty)
	err := c.cc.Invoke(ctx, "/pb.SyncAggregate/SetReceiptLatest", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *syncAggregateClient) AddTyping(ctx context.Context, in *UpdateTypingReq, opts ...grpc.CallOption) (*Empty, error) {
	out := new(Empty)
	err := c.cc.Invoke(ctx, "/pb.SyncAggregate/AddTyping", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *syncAggregateClient) RemoveTyping(ctx context.Context, in *UpdateTypingReq, opts ...grpc.CallOption) (*Empty, error) {
	out := new(Empty)
	err := c.cc.Invoke(ctx, "/pb.SyncAggregate/RemoveTyping", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SyncAggregateServer is the server API for SyncAggregate service.
type SyncAggregateServer interface {
	UpdateOneTimeKey(context.Context, *UpdateOneTimeKeyReq) (*Empty, error)
	UpdateDeviceKey(context.Context, *UpdateDeviceKeyReq) (*Empty, error)
	GetOnlinePresence(context.Context, *GetOnlinePresenceReq) (*GetOnlinePresenceRsp, error)
	SetReceiptLatest(context.Context, *SetReceiptLatestReq) (*Empty, error)
	AddTyping(context.Context, *UpdateTypingReq) (*Empty, error)
	RemoveTyping(context.Context, *UpdateTypingReq) (*Empty, error)
}

// UnimplementedSyncAggregateServer can be embedded to have forward compatible implementations.
type UnimplementedSyncAggregateServer struct {
}

func (*UnimplementedSyncAggregateServer) UpdateOneTimeKey(context.Context, *UpdateOneTimeKeyReq) (*Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateOneTimeKey not implemented")
}
func (*UnimplementedSyncAggregateServer) UpdateDeviceKey(context.Context, *UpdateDeviceKeyReq) (*Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateDeviceKey not implemented")
}
func (*UnimplementedSyncAggregateServer) GetOnlinePresence(context.Context, *GetOnlinePresenceReq) (*GetOnlinePresenceRsp, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetOnlinePresence not implemented")
}
func (*UnimplementedSyncAggregateServer) SetReceiptLatest(context.Context, *SetReceiptLatestReq) (*Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetReceiptLatest not implemented")
}
func (*UnimplementedSyncAggregateServer) AddTyping(context.Context, *UpdateTypingReq) (*Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AddTyping not implemented")
}
func (*UnimplementedSyncAggregateServer) RemoveTyping(context.Context, *UpdateTypingReq) (*Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RemoveTyping not implemented")
}

func RegisterSyncAggregateServer(s *grpc.Server, srv SyncAggregateServer) {
	s.RegisterService(&_SyncAggregate_serviceDesc, srv)
}

func _SyncAggregate_UpdateOneTimeKey_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateOneTimeKeyReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SyncAggregateServer).UpdateOneTimeKey(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/pb.SyncAggregate/UpdateOneTimeKey",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SyncAggregateServer).UpdateOneTimeKey(ctx, req.(*UpdateOneTimeKeyReq))
	}
	return interceptor(ctx, in, info, handler)
}

func _SyncAggregate_UpdateDeviceKey_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateDeviceKeyReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SyncAggregateServer).UpdateDeviceKey(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/pb.SyncAggregate/UpdateDeviceKey",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SyncAggregateServer).UpdateDeviceKey(ctx, req.(*UpdateDeviceKeyReq))
	}
	return interceptor(ctx, in, info, handler)
}

func _SyncAggregate_GetOnlinePresence_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetOnlinePresenceReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SyncAggregateServer).GetOnlinePresence(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/pb.SyncAggregate/GetOnlinePresence",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SyncAggregateServer).GetOnlinePresence(ctx, req.(*GetOnlinePresenceReq))
	}
	return interceptor(ctx, in, info, handler)
}

func _SyncAggregate_SetReceiptLatest_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SetReceiptLatestReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SyncAggregateServer).SetReceiptLatest(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/pb.SyncAggregate/SetReceiptLatest",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SyncAggregateServer).SetReceiptLatest(ctx, req.(*SetReceiptLatestReq))
	}
	return interceptor(ctx, in, info, handler)
}

func _SyncAggregate_AddTyping_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateTypingReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SyncAggregateServer).AddTyping(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/pb.SyncAggregate/AddTyping",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SyncAggregateServer).AddTyping(ctx, req.(*UpdateTypingReq))
	}
	return interceptor(ctx, in, info, handler)
}

func _SyncAggregate_RemoveTyping_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateTypingReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SyncAggregateServer).RemoveTyping(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/pb.SyncAggregate/RemoveTyping",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SyncAggregateServer).RemoveTyping(ctx, req.(*UpdateTypingReq))
	}
	return interceptor(ctx, in, info, handler)
}

var _SyncAggregate_serviceDesc = grpc.ServiceDesc{
	ServiceName: "pb.SyncAggregate",
	HandlerType: (*SyncAggregateServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "UpdateOneTimeKey",
			Handler:    _SyncAggregate_UpdateOneTimeKey_Handler,
		},
		{
			MethodName: "UpdateDeviceKey",
			Handler:    _SyncAggregate_UpdateDeviceKey_Handler,
		},
		{
			MethodName: "GetOnlinePresence",
			Handler:    _SyncAggregate_GetOnlinePresence_Handler,
		},
		{
			MethodName: "SetReceiptLatest",
			Handler:    _SyncAggregate_SetReceiptLatest_Handler,
		},
		{
			MethodName: "AddTyping",
			Handler:    _SyncAggregate_AddTyping_Handler,
		},
		{
			MethodName: "RemoveTyping",
			Handler:    _SyncAggregate_RemoveTyping_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "syncaggregate.proto",
}
