// Code generated by protoc-gen-go. DO NOT EDIT.
// source: grpc/grpc.proto

package grpc // import "github.com/ProtocolONE/payone-billing-service/pkg/proto/grpc"

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"
import billing "github.com/ProtocolONE/payone-billing-service/pkg/proto/billing"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type EmptyRequest struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *EmptyRequest) Reset()         { *m = EmptyRequest{} }
func (m *EmptyRequest) String() string { return proto.CompactTextString(m) }
func (*EmptyRequest) ProtoMessage()    {}
func (*EmptyRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_grpc_05ebc32ed1b7d5a5, []int{0}
}
func (m *EmptyRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_EmptyRequest.Unmarshal(m, b)
}
func (m *EmptyRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_EmptyRequest.Marshal(b, m, deterministic)
}
func (dst *EmptyRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_EmptyRequest.Merge(dst, src)
}
func (m *EmptyRequest) XXX_Size() int {
	return xxx_messageInfo_EmptyRequest.Size(m)
}
func (m *EmptyRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_EmptyRequest.DiscardUnknown(m)
}

var xxx_messageInfo_EmptyRequest proto.InternalMessageInfo

type EmptyResponse struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *EmptyResponse) Reset()         { *m = EmptyResponse{} }
func (m *EmptyResponse) String() string { return proto.CompactTextString(m) }
func (*EmptyResponse) ProtoMessage()    {}
func (*EmptyResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_grpc_05ebc32ed1b7d5a5, []int{1}
}
func (m *EmptyResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_EmptyResponse.Unmarshal(m, b)
}
func (m *EmptyResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_EmptyResponse.Marshal(b, m, deterministic)
}
func (dst *EmptyResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_EmptyResponse.Merge(dst, src)
}
func (m *EmptyResponse) XXX_Size() int {
	return xxx_messageInfo_EmptyResponse.Size(m)
}
func (m *EmptyResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_EmptyResponse.DiscardUnknown(m)
}

var xxx_messageInfo_EmptyResponse proto.InternalMessageInfo

type PaymentCreateRequest struct {
	Data                 map[string]string `protobuf:"bytes,1,rep,name=data,proto3" json:"data,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	XXX_NoUnkeyedLiteral struct{}          `json:"-"`
	XXX_unrecognized     []byte            `json:"-"`
	XXX_sizecache        int32             `json:"-"`
}

func (m *PaymentCreateRequest) Reset()         { *m = PaymentCreateRequest{} }
func (m *PaymentCreateRequest) String() string { return proto.CompactTextString(m) }
func (*PaymentCreateRequest) ProtoMessage()    {}
func (*PaymentCreateRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_grpc_05ebc32ed1b7d5a5, []int{2}
}
func (m *PaymentCreateRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PaymentCreateRequest.Unmarshal(m, b)
}
func (m *PaymentCreateRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PaymentCreateRequest.Marshal(b, m, deterministic)
}
func (dst *PaymentCreateRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PaymentCreateRequest.Merge(dst, src)
}
func (m *PaymentCreateRequest) XXX_Size() int {
	return xxx_messageInfo_PaymentCreateRequest.Size(m)
}
func (m *PaymentCreateRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_PaymentCreateRequest.DiscardUnknown(m)
}

var xxx_messageInfo_PaymentCreateRequest proto.InternalMessageInfo

func (m *PaymentCreateRequest) GetData() map[string]string {
	if m != nil {
		return m.Data
	}
	return nil
}

type PaymentCreateResponse struct {
	Status               int32    `protobuf:"varint,1,opt,name=status,proto3" json:"status,omitempty"`
	RedirectUrl          string   `protobuf:"bytes,2,opt,name=redirect_url,json=redirectUrl,proto3" json:"redirect_url,omitempty"`
	Error                string   `protobuf:"bytes,3,opt,name=error,proto3" json:"error,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *PaymentCreateResponse) Reset()         { *m = PaymentCreateResponse{} }
func (m *PaymentCreateResponse) String() string { return proto.CompactTextString(m) }
func (*PaymentCreateResponse) ProtoMessage()    {}
func (*PaymentCreateResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_grpc_05ebc32ed1b7d5a5, []int{3}
}
func (m *PaymentCreateResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PaymentCreateResponse.Unmarshal(m, b)
}
func (m *PaymentCreateResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PaymentCreateResponse.Marshal(b, m, deterministic)
}
func (dst *PaymentCreateResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PaymentCreateResponse.Merge(dst, src)
}
func (m *PaymentCreateResponse) XXX_Size() int {
	return xxx_messageInfo_PaymentCreateResponse.Size(m)
}
func (m *PaymentCreateResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_PaymentCreateResponse.DiscardUnknown(m)
}

var xxx_messageInfo_PaymentCreateResponse proto.InternalMessageInfo

func (m *PaymentCreateResponse) GetStatus() int32 {
	if m != nil {
		return m.Status
	}
	return 0
}

func (m *PaymentCreateResponse) GetRedirectUrl() string {
	if m != nil {
		return m.RedirectUrl
	}
	return ""
}

func (m *PaymentCreateResponse) GetError() string {
	if m != nil {
		return m.Error
	}
	return ""
}

type PaymentFormJsonDataRequest struct {
	OrderId              string   `protobuf:"bytes,1,opt,name=order_id,json=orderId,proto3" json:"order_id,omitempty"`
	Scheme               string   `protobuf:"bytes,2,opt,name=scheme,proto3" json:"scheme,omitempty"`
	Host                 string   `protobuf:"bytes,3,opt,name=host,proto3" json:"host,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *PaymentFormJsonDataRequest) Reset()         { *m = PaymentFormJsonDataRequest{} }
func (m *PaymentFormJsonDataRequest) String() string { return proto.CompactTextString(m) }
func (*PaymentFormJsonDataRequest) ProtoMessage()    {}
func (*PaymentFormJsonDataRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_grpc_05ebc32ed1b7d5a5, []int{4}
}
func (m *PaymentFormJsonDataRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PaymentFormJsonDataRequest.Unmarshal(m, b)
}
func (m *PaymentFormJsonDataRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PaymentFormJsonDataRequest.Marshal(b, m, deterministic)
}
func (dst *PaymentFormJsonDataRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PaymentFormJsonDataRequest.Merge(dst, src)
}
func (m *PaymentFormJsonDataRequest) XXX_Size() int {
	return xxx_messageInfo_PaymentFormJsonDataRequest.Size(m)
}
func (m *PaymentFormJsonDataRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_PaymentFormJsonDataRequest.DiscardUnknown(m)
}

var xxx_messageInfo_PaymentFormJsonDataRequest proto.InternalMessageInfo

func (m *PaymentFormJsonDataRequest) GetOrderId() string {
	if m != nil {
		return m.OrderId
	}
	return ""
}

func (m *PaymentFormJsonDataRequest) GetScheme() string {
	if m != nil {
		return m.Scheme
	}
	return ""
}

func (m *PaymentFormJsonDataRequest) GetHost() string {
	if m != nil {
		return m.Host
	}
	return ""
}

type PaymentFormJsonDataProject struct {
	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	// @inject_tag: json:"url_success,omitempty"
	UrlSuccess string `protobuf:"bytes,2,opt,name=url_success,json=urlSuccess,proto3" json:"url_success,omitempty"`
	// @inject_tag: json:"url_fail,omitempty"
	UrlFail              string   `protobuf:"bytes,3,opt,name=url_fail,json=urlFail,proto3" json:"url_fail,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *PaymentFormJsonDataProject) Reset()         { *m = PaymentFormJsonDataProject{} }
func (m *PaymentFormJsonDataProject) String() string { return proto.CompactTextString(m) }
func (*PaymentFormJsonDataProject) ProtoMessage()    {}
func (*PaymentFormJsonDataProject) Descriptor() ([]byte, []int) {
	return fileDescriptor_grpc_05ebc32ed1b7d5a5, []int{5}
}
func (m *PaymentFormJsonDataProject) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PaymentFormJsonDataProject.Unmarshal(m, b)
}
func (m *PaymentFormJsonDataProject) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PaymentFormJsonDataProject.Marshal(b, m, deterministic)
}
func (dst *PaymentFormJsonDataProject) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PaymentFormJsonDataProject.Merge(dst, src)
}
func (m *PaymentFormJsonDataProject) XXX_Size() int {
	return xxx_messageInfo_PaymentFormJsonDataProject.Size(m)
}
func (m *PaymentFormJsonDataProject) XXX_DiscardUnknown() {
	xxx_messageInfo_PaymentFormJsonDataProject.DiscardUnknown(m)
}

var xxx_messageInfo_PaymentFormJsonDataProject proto.InternalMessageInfo

func (m *PaymentFormJsonDataProject) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *PaymentFormJsonDataProject) GetUrlSuccess() string {
	if m != nil {
		return m.UrlSuccess
	}
	return ""
}

func (m *PaymentFormJsonDataProject) GetUrlFail() string {
	if m != nil {
		return m.UrlFail
	}
	return ""
}

type PaymentFormJsonDataResponse struct {
	Id string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	// @inject_tag: json:"account,omitempty"
	Account               string                              `protobuf:"bytes,2,opt,name=account,proto3" json:"account,omitempty"`
	HasVat                bool                                `protobuf:"varint,3,opt,name=has_vat,json=hasVat,proto3" json:"has_vat,omitempty"`
	HasUserCommission     bool                                `protobuf:"varint,4,opt,name=has_user_commission,json=hasUserCommission,proto3" json:"has_user_commission,omitempty"`
	Project               *PaymentFormJsonDataProject         `protobuf:"bytes,5,opt,name=project,proto3" json:"project,omitempty"`
	PaymentMethods        []*billing.PaymentFormPaymentMethod `protobuf:"bytes,6,rep,name=payment_methods,json=paymentMethods,proto3" json:"payment_methods,omitempty"`
	InlineFormRedirectUrl string                              `protobuf:"bytes,7,opt,name=inline_form_redirect_url,json=inlineFormRedirectUrl,proto3" json:"inline_form_redirect_url,omitempty"`
	Token                 string                              `protobuf:"bytes,8,opt,name=token,proto3" json:"token,omitempty"`
	XXX_NoUnkeyedLiteral  struct{}                            `json:"-"`
	XXX_unrecognized      []byte                              `json:"-"`
	XXX_sizecache         int32                               `json:"-"`
}

func (m *PaymentFormJsonDataResponse) Reset()         { *m = PaymentFormJsonDataResponse{} }
func (m *PaymentFormJsonDataResponse) String() string { return proto.CompactTextString(m) }
func (*PaymentFormJsonDataResponse) ProtoMessage()    {}
func (*PaymentFormJsonDataResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_grpc_05ebc32ed1b7d5a5, []int{6}
}
func (m *PaymentFormJsonDataResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PaymentFormJsonDataResponse.Unmarshal(m, b)
}
func (m *PaymentFormJsonDataResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PaymentFormJsonDataResponse.Marshal(b, m, deterministic)
}
func (dst *PaymentFormJsonDataResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PaymentFormJsonDataResponse.Merge(dst, src)
}
func (m *PaymentFormJsonDataResponse) XXX_Size() int {
	return xxx_messageInfo_PaymentFormJsonDataResponse.Size(m)
}
func (m *PaymentFormJsonDataResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_PaymentFormJsonDataResponse.DiscardUnknown(m)
}

var xxx_messageInfo_PaymentFormJsonDataResponse proto.InternalMessageInfo

func (m *PaymentFormJsonDataResponse) GetId() string {
	if m != nil {
		return m.Id
	}
	return ""
}

func (m *PaymentFormJsonDataResponse) GetAccount() string {
	if m != nil {
		return m.Account
	}
	return ""
}

func (m *PaymentFormJsonDataResponse) GetHasVat() bool {
	if m != nil {
		return m.HasVat
	}
	return false
}

func (m *PaymentFormJsonDataResponse) GetHasUserCommission() bool {
	if m != nil {
		return m.HasUserCommission
	}
	return false
}

func (m *PaymentFormJsonDataResponse) GetProject() *PaymentFormJsonDataProject {
	if m != nil {
		return m.Project
	}
	return nil
}

func (m *PaymentFormJsonDataResponse) GetPaymentMethods() []*billing.PaymentFormPaymentMethod {
	if m != nil {
		return m.PaymentMethods
	}
	return nil
}

func (m *PaymentFormJsonDataResponse) GetInlineFormRedirectUrl() string {
	if m != nil {
		return m.InlineFormRedirectUrl
	}
	return ""
}

func (m *PaymentFormJsonDataResponse) GetToken() string {
	if m != nil {
		return m.Token
	}
	return ""
}

type PaymentNotifyRequest struct {
	OrderId              string   `protobuf:"bytes,1,opt,name=order_id,json=orderId,proto3" json:"order_id,omitempty"`
	Request              []byte   `protobuf:"bytes,2,opt,name=request,proto3" json:"request,omitempty"`
	Signature            string   `protobuf:"bytes,3,opt,name=signature,proto3" json:"signature,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *PaymentNotifyRequest) Reset()         { *m = PaymentNotifyRequest{} }
func (m *PaymentNotifyRequest) String() string { return proto.CompactTextString(m) }
func (*PaymentNotifyRequest) ProtoMessage()    {}
func (*PaymentNotifyRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_grpc_05ebc32ed1b7d5a5, []int{7}
}
func (m *PaymentNotifyRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PaymentNotifyRequest.Unmarshal(m, b)
}
func (m *PaymentNotifyRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PaymentNotifyRequest.Marshal(b, m, deterministic)
}
func (dst *PaymentNotifyRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PaymentNotifyRequest.Merge(dst, src)
}
func (m *PaymentNotifyRequest) XXX_Size() int {
	return xxx_messageInfo_PaymentNotifyRequest.Size(m)
}
func (m *PaymentNotifyRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_PaymentNotifyRequest.DiscardUnknown(m)
}

var xxx_messageInfo_PaymentNotifyRequest proto.InternalMessageInfo

func (m *PaymentNotifyRequest) GetOrderId() string {
	if m != nil {
		return m.OrderId
	}
	return ""
}

func (m *PaymentNotifyRequest) GetRequest() []byte {
	if m != nil {
		return m.Request
	}
	return nil
}

func (m *PaymentNotifyRequest) GetSignature() string {
	if m != nil {
		return m.Signature
	}
	return ""
}

type PaymentNotifyResponse struct {
	Status               int32    `protobuf:"varint,1,opt,name=status,proto3" json:"status,omitempty"`
	Error                string   `protobuf:"bytes,2,opt,name=error,proto3" json:"error,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *PaymentNotifyResponse) Reset()         { *m = PaymentNotifyResponse{} }
func (m *PaymentNotifyResponse) String() string { return proto.CompactTextString(m) }
func (*PaymentNotifyResponse) ProtoMessage()    {}
func (*PaymentNotifyResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_grpc_05ebc32ed1b7d5a5, []int{8}
}
func (m *PaymentNotifyResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PaymentNotifyResponse.Unmarshal(m, b)
}
func (m *PaymentNotifyResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PaymentNotifyResponse.Marshal(b, m, deterministic)
}
func (dst *PaymentNotifyResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PaymentNotifyResponse.Merge(dst, src)
}
func (m *PaymentNotifyResponse) XXX_Size() int {
	return xxx_messageInfo_PaymentNotifyResponse.Size(m)
}
func (m *PaymentNotifyResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_PaymentNotifyResponse.DiscardUnknown(m)
}

var xxx_messageInfo_PaymentNotifyResponse proto.InternalMessageInfo

func (m *PaymentNotifyResponse) GetStatus() int32 {
	if m != nil {
		return m.Status
	}
	return 0
}

func (m *PaymentNotifyResponse) GetError() string {
	if m != nil {
		return m.Error
	}
	return ""
}

func init() {
	proto.RegisterType((*EmptyRequest)(nil), "grpc.EmptyRequest")
	proto.RegisterType((*EmptyResponse)(nil), "grpc.EmptyResponse")
	proto.RegisterType((*PaymentCreateRequest)(nil), "grpc.PaymentCreateRequest")
	proto.RegisterMapType((map[string]string)(nil), "grpc.PaymentCreateRequest.DataEntry")
	proto.RegisterType((*PaymentCreateResponse)(nil), "grpc.PaymentCreateResponse")
	proto.RegisterType((*PaymentFormJsonDataRequest)(nil), "grpc.PaymentFormJsonDataRequest")
	proto.RegisterType((*PaymentFormJsonDataProject)(nil), "grpc.PaymentFormJsonDataProject")
	proto.RegisterType((*PaymentFormJsonDataResponse)(nil), "grpc.PaymentFormJsonDataResponse")
	proto.RegisterType((*PaymentNotifyRequest)(nil), "grpc.PaymentNotifyRequest")
	proto.RegisterType((*PaymentNotifyResponse)(nil), "grpc.PaymentNotifyResponse")
}

func init() { proto.RegisterFile("grpc/grpc.proto", fileDescriptor_grpc_05ebc32ed1b7d5a5) }

var fileDescriptor_grpc_05ebc32ed1b7d5a5 = []byte{
	// 745 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x8c, 0x55, 0xdd, 0x6e, 0xfb, 0x34,
	0x14, 0x5f, 0xbb, 0xae, 0xdd, 0x4e, 0x4b, 0xc7, 0xbc, 0x0f, 0x42, 0x87, 0x44, 0x17, 0x71, 0xb1,
	0x9b, 0xb5, 0xd2, 0x40, 0xda, 0x98, 0x10, 0x17, 0x2b, 0x9d, 0xc4, 0xa4, 0x6d, 0x25, 0xd3, 0xb8,
	0xe0, 0x26, 0x72, 0x1d, 0xaf, 0x31, 0x75, 0xe2, 0x60, 0x3b, 0x93, 0xfa, 0x08, 0x5c, 0xf1, 0x98,
	0xbc, 0x06, 0x8a, 0xed, 0x84, 0xb6, 0x6a, 0xe1, 0x7f, 0xd3, 0x9e, 0xef, 0xdf, 0x39, 0xc7, 0x3f,
	0x3b, 0x70, 0x38, 0x93, 0x19, 0x19, 0x16, 0x3f, 0x83, 0x4c, 0x0a, 0x2d, 0x50, 0xa3, 0x90, 0x7b,
	0xa7, 0x53, 0xc6, 0x39, 0x4b, 0x67, 0x43, 0xf7, 0x6f, 0x9d, 0x7e, 0x17, 0x3a, 0xe3, 0x24, 0xd3,
	0x8b, 0x80, 0xfe, 0x91, 0x53, 0xa5, 0xfd, 0x43, 0xf8, 0xcc, 0xe9, 0x2a, 0x13, 0xa9, 0xa2, 0xfe,
	0x9f, 0x35, 0x38, 0x99, 0xe0, 0x45, 0x42, 0x53, 0x3d, 0x92, 0x14, 0x6b, 0xea, 0x22, 0xd1, 0x2d,
	0x34, 0x22, 0xac, 0xb1, 0x57, 0xeb, 0xef, 0x5e, 0xb6, 0xaf, 0xbf, 0x19, 0x18, 0xc4, 0x4d, 0x91,
	0x83, 0x9f, 0xb0, 0xc6, 0xe3, 0x54, 0xcb, 0x45, 0x60, 0x32, 0x7a, 0x37, 0x70, 0x50, 0x99, 0xd0,
	0xe7, 0xb0, 0x3b, 0xa7, 0x0b, 0xaf, 0xd6, 0xaf, 0x5d, 0x1e, 0x04, 0x85, 0x88, 0x4e, 0x60, 0xef,
	0x03, 0xf3, 0x9c, 0x7a, 0x75, 0x63, 0xb3, 0xca, 0x5d, 0xfd, 0xb6, 0xe6, 0xc7, 0x70, 0xba, 0x06,
	0x60, 0x9b, 0x44, 0x67, 0xd0, 0x54, 0x1a, 0xeb, 0x5c, 0x99, 0x3a, 0x7b, 0x81, 0xd3, 0xd0, 0x05,
	0x74, 0x24, 0x8d, 0x98, 0xa4, 0x44, 0x87, 0xb9, 0xe4, 0xae, 0x62, 0xbb, 0xb4, 0xbd, 0x49, 0x5e,
	0xa0, 0x51, 0x29, 0x85, 0xf4, 0x76, 0x2d, 0x9a, 0x51, 0x7c, 0x02, 0x3d, 0x87, 0xf4, 0x20, 0x64,
	0xf2, 0xa8, 0x44, 0x5a, 0x74, 0x5c, 0x8e, 0xfe, 0x25, 0xec, 0x0b, 0x19, 0x51, 0x19, 0xb2, 0xc8,
	0x35, 0xde, 0x32, 0xfa, 0xcf, 0x91, 0xe9, 0x84, 0xc4, 0x34, 0x29, 0xbb, 0x77, 0x1a, 0x42, 0xd0,
	0x88, 0x85, 0xd2, 0x0e, 0xc5, 0xc8, 0x3e, 0xdf, 0x08, 0x32, 0x91, 0xe2, 0x77, 0x4a, 0x74, 0x91,
	0x91, 0xe2, 0x84, 0x3a, 0x00, 0x23, 0xa3, 0xaf, 0xa1, 0x9d, 0x4b, 0x1e, 0xaa, 0x9c, 0x10, 0xaa,
	0x94, 0x83, 0x80, 0x5c, 0xf2, 0x57, 0x6b, 0x29, 0x3a, 0x2b, 0x02, 0xde, 0x31, 0xe3, 0x0e, 0xaa,
	0x95, 0x4b, 0xfe, 0x80, 0x19, 0xf7, 0xff, 0xae, 0xc3, 0xf9, 0xc6, 0x99, 0xdc, 0x0e, 0xbb, 0x50,
	0xaf, 0xc6, 0xa9, 0xb3, 0x08, 0x79, 0xd0, 0xc2, 0x84, 0x88, 0x3c, 0xd5, 0x0e, 0xa7, 0x54, 0xd1,
	0x17, 0xd0, 0x8a, 0xb1, 0x0a, 0x3f, 0xb0, 0x1d, 0x67, 0x3f, 0x68, 0xc6, 0x58, 0xfd, 0x8a, 0x35,
	0x1a, 0xc0, 0x71, 0xe1, 0xc8, 0x15, 0x95, 0x21, 0x11, 0x49, 0xc2, 0x94, 0x62, 0x22, 0xf5, 0x1a,
	0x26, 0xe8, 0x28, 0xc6, 0xea, 0x4d, 0x51, 0x39, 0xaa, 0x1c, 0xe8, 0x0e, 0x5a, 0x99, 0x9d, 0xd6,
	0xdb, 0xeb, 0xd7, 0x2e, 0xdb, 0xd7, 0xfd, 0x15, 0x16, 0x6d, 0xd8, 0x4a, 0x50, 0x26, 0xa0, 0x47,
	0x38, 0xcc, 0x6c, 0x58, 0x98, 0x50, 0x1d, 0x8b, 0x48, 0x79, 0x4d, 0xc3, 0xc4, 0x8b, 0x41, 0xc9,
	0xf0, 0xa5, 0x32, 0x4e, 0x7c, 0x32, 0x91, 0x41, 0x37, 0x5b, 0x56, 0x15, 0xba, 0x01, 0x8f, 0xa5,
	0x9c, 0xa5, 0x34, 0x7c, 0x17, 0x32, 0x09, 0x57, 0x28, 0xd3, 0x32, 0xb3, 0x9f, 0x5a, 0x7f, 0x51,
	0x2a, 0x58, 0x25, 0x8f, 0x16, 0x73, 0x9a, 0x7a, 0xfb, 0x96, 0x3c, 0x46, 0xf1, 0x59, 0x75, 0x63,
	0x9e, 0x85, 0x66, 0xef, 0x8b, 0x4f, 0xa0, 0x8d, 0x07, 0x2d, 0x69, 0xa3, 0xcc, 0xb2, 0x3b, 0x41,
	0xa9, 0xa2, 0xaf, 0xe0, 0x40, 0xb1, 0x59, 0x8a, 0x75, 0x2e, 0xa9, 0x3b, 0xd2, 0x7f, 0x0d, 0xfe,
	0xb8, 0xba, 0x11, 0x25, 0xd4, 0xff, 0xdc, 0x88, 0x8a, 0xee, 0xf5, 0x25, 0xba, 0x5f, 0xff, 0xd5,
	0x80, 0xee, 0xbd, 0xdd, 0xda, 0x2b, 0x95, 0x1f, 0x8c, 0x50, 0x34, 0x02, 0xf4, 0x52, 0x34, 0x67,
	0x6f, 0xda, 0x44, 0x0a, 0xc3, 0xaf, 0xf3, 0x6a, 0xb9, 0x4b, 0x4e, 0x37, 0x5f, 0xaf, 0xbb, 0xea,
	0xf4, 0x77, 0x10, 0xd9, 0xc6, 0x70, 0x53, 0x6c, 0xfb, 0x69, 0x97, 0x15, 0x2f, 0xfe, 0x23, 0xc2,
	0xbd, 0x4f, 0x3b, 0xe8, 0x97, 0xb5, 0x07, 0xaa, 0x2c, 0xdf, 0xdb, 0xfe, 0x24, 0xf5, 0xce, 0x37,
	0xfa, 0xaa, 0x92, 0xaf, 0x70, 0x56, 0xba, 0x30, 0xe7, 0x53, 0x4c, 0xe6, 0x9b, 0x8b, 0xae, 0x9c,
	0xef, 0x5a, 0xd1, 0xd5, 0x03, 0xf1, 0x77, 0xd0, 0xf7, 0xd0, 0x09, 0xe8, 0x34, 0x67, 0x3c, 0x1a,
	0x61, 0x12, 0x53, 0x84, 0x6c, 0xf8, 0xf2, 0xf3, 0xdb, 0x3b, 0x5e, 0xb1, 0x55, 0xa9, 0xdf, 0x41,
	0xfb, 0x2d, 0x8b, 0xb0, 0xa6, 0x66, 0xb1, 0x68, 0x6d, 0xd1, 0xdb, 0xb2, 0xee, 0xa0, 0x6b, 0xb3,
	0x9e, 0xa8, 0x24, 0x31, 0x4e, 0x35, 0x3a, 0xaa, 0x12, 0x4b, 0xd3, 0x96, 0xdc, 0xfb, 0x1f, 0x7f,
	0xfb, 0x61, 0xc6, 0x74, 0x9c, 0x4f, 0x07, 0x44, 0x24, 0xc3, 0x49, 0xf1, 0xad, 0x20, 0x82, 0xbf,
	0x3c, 0x8f, 0x87, 0x19, 0x5e, 0x88, 0x94, 0x5e, 0xb9, 0x42, 0x57, 0xca, 0xf2, 0x65, 0x98, 0xcd,
	0x67, 0x43, 0xf3, 0x49, 0x31, 0x9f, 0x9e, 0x69, 0xd3, 0xc8, 0xdf, 0xfe, 0x13, 0x00, 0x00, 0xff,
	0xff, 0x08, 0x7b, 0x1a, 0x29, 0x8e, 0x06, 0x00, 0x00,
}
