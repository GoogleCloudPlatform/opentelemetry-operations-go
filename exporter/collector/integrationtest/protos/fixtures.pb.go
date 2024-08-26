// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.33.0
// 	protoc        v3.19.0
// source: fixtures.proto

package protos

import (
	loggingpb "cloud.google.com/go/logging/apiv2/loggingpb"
	monitoringpb "cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	tracepb "cloud.google.com/go/trace/apiv2/tracepb"
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

type MetricExpectFixture struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	CreateTimeSeriesRequests        []*monitoringpb.CreateTimeSeriesRequest       `protobuf:"bytes,1,rep,name=create_time_series_requests,json=createTimeSeriesRequests,proto3" json:"create_time_series_requests,omitempty"`
	CreateMetricDescriptorRequests  []*monitoringpb.CreateMetricDescriptorRequest `protobuf:"bytes,2,rep,name=create_metric_descriptor_requests,json=createMetricDescriptorRequests,proto3" json:"create_metric_descriptor_requests,omitempty"`
	CreateServiceTimeSeriesRequests []*monitoringpb.CreateTimeSeriesRequest       `protobuf:"bytes,3,rep,name=create_service_time_series_requests,json=createServiceTimeSeriesRequests,proto3" json:"create_service_time_series_requests,omitempty"`
	SelfObservabilityMetrics        *SelfObservabilityMetric                      `protobuf:"bytes,4,opt,name=self_observability_metrics,json=selfObservabilityMetrics,proto3" json:"self_observability_metrics,omitempty"`
	UserAgent                       string                                        `protobuf:"bytes,5,opt,name=user_agent,json=userAgent,proto3" json:"user_agent,omitempty"`
}

func (x *MetricExpectFixture) Reset() {
	*x = MetricExpectFixture{}
	if protoimpl.UnsafeEnabled {
		mi := &file_fixtures_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *MetricExpectFixture) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MetricExpectFixture) ProtoMessage() {}

func (x *MetricExpectFixture) ProtoReflect() protoreflect.Message {
	mi := &file_fixtures_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use MetricExpectFixture.ProtoReflect.Descriptor instead.
func (*MetricExpectFixture) Descriptor() ([]byte, []int) {
	return file_fixtures_proto_rawDescGZIP(), []int{0}
}

func (x *MetricExpectFixture) GetCreateTimeSeriesRequests() []*monitoringpb.CreateTimeSeriesRequest {
	if x != nil {
		return x.CreateTimeSeriesRequests
	}
	return nil
}

func (x *MetricExpectFixture) GetCreateMetricDescriptorRequests() []*monitoringpb.CreateMetricDescriptorRequest {
	if x != nil {
		return x.CreateMetricDescriptorRequests
	}
	return nil
}

func (x *MetricExpectFixture) GetCreateServiceTimeSeriesRequests() []*monitoringpb.CreateTimeSeriesRequest {
	if x != nil {
		return x.CreateServiceTimeSeriesRequests
	}
	return nil
}

func (x *MetricExpectFixture) GetSelfObservabilityMetrics() *SelfObservabilityMetric {
	if x != nil {
		return x.SelfObservabilityMetrics
	}
	return nil
}

func (x *MetricExpectFixture) GetUserAgent() string {
	if x != nil {
		return x.UserAgent
	}
	return ""
}

type SelfObservabilityMetric struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	CreateTimeSeriesRequests       []*monitoringpb.CreateTimeSeriesRequest       `protobuf:"bytes,1,rep,name=create_time_series_requests,json=createTimeSeriesRequests,proto3" json:"create_time_series_requests,omitempty"`
	CreateMetricDescriptorRequests []*monitoringpb.CreateMetricDescriptorRequest `protobuf:"bytes,2,rep,name=create_metric_descriptor_requests,json=createMetricDescriptorRequests,proto3" json:"create_metric_descriptor_requests,omitempty"`
}

func (x *SelfObservabilityMetric) Reset() {
	*x = SelfObservabilityMetric{}
	if protoimpl.UnsafeEnabled {
		mi := &file_fixtures_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SelfObservabilityMetric) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SelfObservabilityMetric) ProtoMessage() {}

func (x *SelfObservabilityMetric) ProtoReflect() protoreflect.Message {
	mi := &file_fixtures_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SelfObservabilityMetric.ProtoReflect.Descriptor instead.
func (*SelfObservabilityMetric) Descriptor() ([]byte, []int) {
	return file_fixtures_proto_rawDescGZIP(), []int{1}
}

func (x *SelfObservabilityMetric) GetCreateTimeSeriesRequests() []*monitoringpb.CreateTimeSeriesRequest {
	if x != nil {
		return x.CreateTimeSeriesRequests
	}
	return nil
}

func (x *SelfObservabilityMetric) GetCreateMetricDescriptorRequests() []*monitoringpb.CreateMetricDescriptorRequest {
	if x != nil {
		return x.CreateMetricDescriptorRequests
	}
	return nil
}

type LogExpectFixture struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	WriteLogEntriesRequests  []*loggingpb.WriteLogEntriesRequest `protobuf:"bytes,1,rep,name=write_log_entries_requests,json=writeLogEntriesRequests,proto3" json:"write_log_entries_requests,omitempty"`
	UserAgent                string                              `protobuf:"bytes,2,opt,name=user_agent,json=userAgent,proto3" json:"user_agent,omitempty"`
	SelfObservabilityMetrics *SelfObservabilityMetric            `protobuf:"bytes,3,opt,name=self_observability_metrics,json=selfObservabilityMetrics,proto3" json:"self_observability_metrics,omitempty"`
}

func (x *LogExpectFixture) Reset() {
	*x = LogExpectFixture{}
	if protoimpl.UnsafeEnabled {
		mi := &file_fixtures_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *LogExpectFixture) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*LogExpectFixture) ProtoMessage() {}

func (x *LogExpectFixture) ProtoReflect() protoreflect.Message {
	mi := &file_fixtures_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use LogExpectFixture.ProtoReflect.Descriptor instead.
func (*LogExpectFixture) Descriptor() ([]byte, []int) {
	return file_fixtures_proto_rawDescGZIP(), []int{2}
}

func (x *LogExpectFixture) GetWriteLogEntriesRequests() []*loggingpb.WriteLogEntriesRequest {
	if x != nil {
		return x.WriteLogEntriesRequests
	}
	return nil
}

func (x *LogExpectFixture) GetUserAgent() string {
	if x != nil {
		return x.UserAgent
	}
	return ""
}

func (x *LogExpectFixture) GetSelfObservabilityMetrics() *SelfObservabilityMetric {
	if x != nil {
		return x.SelfObservabilityMetrics
	}
	return nil
}

type TraceExpectFixture struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	BatchWriteSpansRequest   []*tracepb.BatchWriteSpansRequest `protobuf:"bytes,1,rep,name=batch_write_spans_request,json=batchWriteSpansRequest,proto3" json:"batch_write_spans_request,omitempty"`
	UserAgent                string                            `protobuf:"bytes,2,opt,name=user_agent,json=userAgent,proto3" json:"user_agent,omitempty"`
	SelfObservabilityMetrics *SelfObservabilityMetric          `protobuf:"bytes,3,opt,name=self_observability_metrics,json=selfObservabilityMetrics,proto3" json:"self_observability_metrics,omitempty"`
}

func (x *TraceExpectFixture) Reset() {
	*x = TraceExpectFixture{}
	if protoimpl.UnsafeEnabled {
		mi := &file_fixtures_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TraceExpectFixture) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TraceExpectFixture) ProtoMessage() {}

func (x *TraceExpectFixture) ProtoReflect() protoreflect.Message {
	mi := &file_fixtures_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TraceExpectFixture.ProtoReflect.Descriptor instead.
func (*TraceExpectFixture) Descriptor() ([]byte, []int) {
	return file_fixtures_proto_rawDescGZIP(), []int{3}
}

func (x *TraceExpectFixture) GetBatchWriteSpansRequest() []*tracepb.BatchWriteSpansRequest {
	if x != nil {
		return x.BatchWriteSpansRequest
	}
	return nil
}

func (x *TraceExpectFixture) GetUserAgent() string {
	if x != nil {
		return x.UserAgent
	}
	return ""
}

func (x *TraceExpectFixture) GetSelfObservabilityMetrics() *SelfObservabilityMetric {
	if x != nil {
		return x.SelfObservabilityMetrics
	}
	return nil
}

var File_fixtures_proto protoreflect.FileDescriptor

var file_fixtures_proto_rawDesc = []byte{
	0x0a, 0x0e, 0x66, 0x69, 0x78, 0x74, 0x75, 0x72, 0x65, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x12, 0x08, 0x66, 0x69, 0x78, 0x74, 0x75, 0x72, 0x65, 0x73, 0x1a, 0x14, 0x6d, 0x65, 0x74, 0x72,
	0x69, 0x63, 0x5f, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x1a, 0x15, 0x6c, 0x6f, 0x67, 0x67, 0x69, 0x6e, 0x67, 0x5f, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63,
	0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x15, 0x74, 0x72, 0x61, 0x63, 0x69, 0x6e, 0x67,
	0x5f, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x80,
	0x04, 0x0a, 0x13, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x45, 0x78, 0x70, 0x65, 0x63, 0x74, 0x46,
	0x69, 0x78, 0x74, 0x75, 0x72, 0x65, 0x12, 0x6c, 0x0a, 0x1b, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65,
	0x5f, 0x74, 0x69, 0x6d, 0x65, 0x5f, 0x73, 0x65, 0x72, 0x69, 0x65, 0x73, 0x5f, 0x72, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x2d, 0x2e, 0x67, 0x6f,
	0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x6d, 0x6f, 0x6e, 0x69, 0x74, 0x6f, 0x72, 0x69, 0x6e, 0x67, 0x2e,
	0x76, 0x33, 0x2e, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x54, 0x69, 0x6d, 0x65, 0x53, 0x65, 0x72,
	0x69, 0x65, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x52, 0x18, 0x63, 0x72, 0x65, 0x61,
	0x74, 0x65, 0x54, 0x69, 0x6d, 0x65, 0x53, 0x65, 0x72, 0x69, 0x65, 0x73, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x73, 0x12, 0x7e, 0x0a, 0x21, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x5f, 0x6d,
	0x65, 0x74, 0x72, 0x69, 0x63, 0x5f, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x6f, 0x72,
	0x5f, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32,
	0x33, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x6d, 0x6f, 0x6e, 0x69, 0x74, 0x6f, 0x72,
	0x69, 0x6e, 0x67, 0x2e, 0x76, 0x33, 0x2e, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x4d, 0x65, 0x74,
	0x72, 0x69, 0x63, 0x44, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x6f, 0x72, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x52, 0x1e, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x4d, 0x65, 0x74, 0x72,
	0x69, 0x63, 0x44, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x6f, 0x72, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x73, 0x12, 0x7b, 0x0a, 0x23, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x5f, 0x73,
	0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x5f, 0x74, 0x69, 0x6d, 0x65, 0x5f, 0x73, 0x65, 0x72, 0x69,
	0x65, 0x73, 0x5f, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x73, 0x18, 0x03, 0x20, 0x03, 0x28,
	0x0b, 0x32, 0x2d, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x6d, 0x6f, 0x6e, 0x69, 0x74,
	0x6f, 0x72, 0x69, 0x6e, 0x67, 0x2e, 0x76, 0x33, 0x2e, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x54,
	0x69, 0x6d, 0x65, 0x53, 0x65, 0x72, 0x69, 0x65, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x52, 0x1f, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x54,
	0x69, 0x6d, 0x65, 0x53, 0x65, 0x72, 0x69, 0x65, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x73, 0x12, 0x5f, 0x0a, 0x1a, 0x73, 0x65, 0x6c, 0x66, 0x5f, 0x6f, 0x62, 0x73, 0x65, 0x72, 0x76,
	0x61, 0x62, 0x69, 0x6c, 0x69, 0x74, 0x79, 0x5f, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x18,
	0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x21, 0x2e, 0x66, 0x69, 0x78, 0x74, 0x75, 0x72, 0x65, 0x73,
	0x2e, 0x53, 0x65, 0x6c, 0x66, 0x4f, 0x62, 0x73, 0x65, 0x72, 0x76, 0x61, 0x62, 0x69, 0x6c, 0x69,
	0x74, 0x79, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x52, 0x18, 0x73, 0x65, 0x6c, 0x66, 0x4f, 0x62,
	0x73, 0x65, 0x72, 0x76, 0x61, 0x62, 0x69, 0x6c, 0x69, 0x74, 0x79, 0x4d, 0x65, 0x74, 0x72, 0x69,
	0x63, 0x73, 0x12, 0x1d, 0x0a, 0x0a, 0x75, 0x73, 0x65, 0x72, 0x5f, 0x61, 0x67, 0x65, 0x6e, 0x74,
	0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x75, 0x73, 0x65, 0x72, 0x41, 0x67, 0x65, 0x6e,
	0x74, 0x22, 0x87, 0x02, 0x0a, 0x17, 0x53, 0x65, 0x6c, 0x66, 0x4f, 0x62, 0x73, 0x65, 0x72, 0x76,
	0x61, 0x62, 0x69, 0x6c, 0x69, 0x74, 0x79, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x12, 0x6c, 0x0a,
	0x1b, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x5f, 0x74, 0x69, 0x6d, 0x65, 0x5f, 0x73, 0x65, 0x72,
	0x69, 0x65, 0x73, 0x5f, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x73, 0x18, 0x01, 0x20, 0x03,
	0x28, 0x0b, 0x32, 0x2d, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x6d, 0x6f, 0x6e, 0x69,
	0x74, 0x6f, 0x72, 0x69, 0x6e, 0x67, 0x2e, 0x76, 0x33, 0x2e, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65,
	0x54, 0x69, 0x6d, 0x65, 0x53, 0x65, 0x72, 0x69, 0x65, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x52, 0x18, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x54, 0x69, 0x6d, 0x65, 0x53, 0x65, 0x72,
	0x69, 0x65, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x73, 0x12, 0x7e, 0x0a, 0x21, 0x63,
	0x72, 0x65, 0x61, 0x74, 0x65, 0x5f, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x5f, 0x64, 0x65, 0x73,
	0x63, 0x72, 0x69, 0x70, 0x74, 0x6f, 0x72, 0x5f, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x73,
	0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x33, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e,
	0x6d, 0x6f, 0x6e, 0x69, 0x74, 0x6f, 0x72, 0x69, 0x6e, 0x67, 0x2e, 0x76, 0x33, 0x2e, 0x43, 0x72,
	0x65, 0x61, 0x74, 0x65, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x44, 0x65, 0x73, 0x63, 0x72, 0x69,
	0x70, 0x74, 0x6f, 0x72, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x52, 0x1e, 0x63, 0x72, 0x65,
	0x61, 0x74, 0x65, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x44, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70,
	0x74, 0x6f, 0x72, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x73, 0x22, 0xfa, 0x01, 0x0a, 0x10,
	0x4c, 0x6f, 0x67, 0x45, 0x78, 0x70, 0x65, 0x63, 0x74, 0x46, 0x69, 0x78, 0x74, 0x75, 0x72, 0x65,
	0x12, 0x66, 0x0a, 0x1a, 0x77, 0x72, 0x69, 0x74, 0x65, 0x5f, 0x6c, 0x6f, 0x67, 0x5f, 0x65, 0x6e,
	0x74, 0x72, 0x69, 0x65, 0x73, 0x5f, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x73, 0x18, 0x01,
	0x20, 0x03, 0x28, 0x0b, 0x32, 0x29, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x6c, 0x6f,
	0x67, 0x67, 0x69, 0x6e, 0x67, 0x2e, 0x76, 0x32, 0x2e, 0x57, 0x72, 0x69, 0x74, 0x65, 0x4c, 0x6f,
	0x67, 0x45, 0x6e, 0x74, 0x72, 0x69, 0x65, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x52,
	0x17, 0x77, 0x72, 0x69, 0x74, 0x65, 0x4c, 0x6f, 0x67, 0x45, 0x6e, 0x74, 0x72, 0x69, 0x65, 0x73,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x73, 0x12, 0x1d, 0x0a, 0x0a, 0x75, 0x73, 0x65, 0x72,
	0x5f, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x75, 0x73,
	0x65, 0x72, 0x41, 0x67, 0x65, 0x6e, 0x74, 0x12, 0x5f, 0x0a, 0x1a, 0x73, 0x65, 0x6c, 0x66, 0x5f,
	0x6f, 0x62, 0x73, 0x65, 0x72, 0x76, 0x61, 0x62, 0x69, 0x6c, 0x69, 0x74, 0x79, 0x5f, 0x6d, 0x65,
	0x74, 0x72, 0x69, 0x63, 0x73, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x21, 0x2e, 0x66, 0x69,
	0x78, 0x74, 0x75, 0x72, 0x65, 0x73, 0x2e, 0x53, 0x65, 0x6c, 0x66, 0x4f, 0x62, 0x73, 0x65, 0x72,
	0x76, 0x61, 0x62, 0x69, 0x6c, 0x69, 0x74, 0x79, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x52, 0x18,
	0x73, 0x65, 0x6c, 0x66, 0x4f, 0x62, 0x73, 0x65, 0x72, 0x76, 0x61, 0x62, 0x69, 0x6c, 0x69, 0x74,
	0x79, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x22, 0xfa, 0x01, 0x0a, 0x12, 0x54, 0x72, 0x61,
	0x63, 0x65, 0x45, 0x78, 0x70, 0x65, 0x63, 0x74, 0x46, 0x69, 0x78, 0x74, 0x75, 0x72, 0x65, 0x12,
	0x64, 0x0a, 0x19, 0x62, 0x61, 0x74, 0x63, 0x68, 0x5f, 0x77, 0x72, 0x69, 0x74, 0x65, 0x5f, 0x73,
	0x70, 0x61, 0x6e, 0x73, 0x5f, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x18, 0x01, 0x20, 0x03,
	0x28, 0x0b, 0x32, 0x29, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x74, 0x72, 0x61, 0x63,
	0x69, 0x6e, 0x67, 0x2e, 0x76, 0x32, 0x2e, 0x42, 0x61, 0x74, 0x63, 0x68, 0x57, 0x72, 0x69, 0x74,
	0x65, 0x53, 0x70, 0x61, 0x6e, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x52, 0x16, 0x62,
	0x61, 0x74, 0x63, 0x68, 0x57, 0x72, 0x69, 0x74, 0x65, 0x53, 0x70, 0x61, 0x6e, 0x73, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1d, 0x0a, 0x0a, 0x75, 0x73, 0x65, 0x72, 0x5f, 0x61, 0x67,
	0x65, 0x6e, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x75, 0x73, 0x65, 0x72, 0x41,
	0x67, 0x65, 0x6e, 0x74, 0x12, 0x5f, 0x0a, 0x1a, 0x73, 0x65, 0x6c, 0x66, 0x5f, 0x6f, 0x62, 0x73,
	0x65, 0x72, 0x76, 0x61, 0x62, 0x69, 0x6c, 0x69, 0x74, 0x79, 0x5f, 0x6d, 0x65, 0x74, 0x72, 0x69,
	0x63, 0x73, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x21, 0x2e, 0x66, 0x69, 0x78, 0x74, 0x75,
	0x72, 0x65, 0x73, 0x2e, 0x53, 0x65, 0x6c, 0x66, 0x4f, 0x62, 0x73, 0x65, 0x72, 0x76, 0x61, 0x62,
	0x69, 0x6c, 0x69, 0x74, 0x79, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x52, 0x18, 0x73, 0x65, 0x6c,
	0x66, 0x4f, 0x62, 0x73, 0x65, 0x72, 0x76, 0x61, 0x62, 0x69, 0x6c, 0x69, 0x74, 0x79, 0x4d, 0x65,
	0x74, 0x72, 0x69, 0x63, 0x73, 0x42, 0x66, 0x5a, 0x64, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e,
	0x63, 0x6f, 0x6d, 0x2f, 0x47, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x43, 0x6c, 0x6f, 0x75, 0x64, 0x50,
	0x6c, 0x61, 0x74, 0x66, 0x6f, 0x72, 0x6d, 0x2f, 0x6f, 0x70, 0x65, 0x6e, 0x74, 0x65, 0x6c, 0x65,
	0x6d, 0x65, 0x74, 0x72, 0x79, 0x2d, 0x6f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73,
	0x2d, 0x67, 0x6f, 0x2f, 0x65, 0x78, 0x70, 0x6f, 0x72, 0x74, 0x65, 0x72, 0x2f, 0x63, 0x6f, 0x6c,
	0x6c, 0x65, 0x63, 0x74, 0x6f, 0x72, 0x2f, 0x69, 0x6e, 0x74, 0x65, 0x67, 0x72, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x74, 0x65, 0x73, 0x74, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x73, 0x62, 0x06, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_fixtures_proto_rawDescOnce sync.Once
	file_fixtures_proto_rawDescData = file_fixtures_proto_rawDesc
)

func file_fixtures_proto_rawDescGZIP() []byte {
	file_fixtures_proto_rawDescOnce.Do(func() {
		file_fixtures_proto_rawDescData = protoimpl.X.CompressGZIP(file_fixtures_proto_rawDescData)
	})
	return file_fixtures_proto_rawDescData
}

var file_fixtures_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_fixtures_proto_goTypes = []interface{}{
	(*MetricExpectFixture)(nil),                        // 0: fixtures.MetricExpectFixture
	(*SelfObservabilityMetric)(nil),                    // 1: fixtures.SelfObservabilityMetric
	(*LogExpectFixture)(nil),                           // 2: fixtures.LogExpectFixture
	(*TraceExpectFixture)(nil),                         // 3: fixtures.TraceExpectFixture
	(*monitoringpb.CreateTimeSeriesRequest)(nil),       // 4: google.monitoring.v3.CreateTimeSeriesRequest
	(*monitoringpb.CreateMetricDescriptorRequest)(nil), // 5: google.monitoring.v3.CreateMetricDescriptorRequest
	(*loggingpb.WriteLogEntriesRequest)(nil),           // 6: google.logging.v2.WriteLogEntriesRequest
	(*tracepb.BatchWriteSpansRequest)(nil),             // 7: google.tracing.v2.BatchWriteSpansRequest
}
var file_fixtures_proto_depIdxs = []int32{
	4,  // 0: fixtures.MetricExpectFixture.create_time_series_requests:type_name -> google.monitoring.v3.CreateTimeSeriesRequest
	5,  // 1: fixtures.MetricExpectFixture.create_metric_descriptor_requests:type_name -> google.monitoring.v3.CreateMetricDescriptorRequest
	4,  // 2: fixtures.MetricExpectFixture.create_service_time_series_requests:type_name -> google.monitoring.v3.CreateTimeSeriesRequest
	1,  // 3: fixtures.MetricExpectFixture.self_observability_metrics:type_name -> fixtures.SelfObservabilityMetric
	4,  // 4: fixtures.SelfObservabilityMetric.create_time_series_requests:type_name -> google.monitoring.v3.CreateTimeSeriesRequest
	5,  // 5: fixtures.SelfObservabilityMetric.create_metric_descriptor_requests:type_name -> google.monitoring.v3.CreateMetricDescriptorRequest
	6,  // 6: fixtures.LogExpectFixture.write_log_entries_requests:type_name -> google.logging.v2.WriteLogEntriesRequest
	1,  // 7: fixtures.LogExpectFixture.self_observability_metrics:type_name -> fixtures.SelfObservabilityMetric
	7,  // 8: fixtures.TraceExpectFixture.batch_write_spans_request:type_name -> google.tracing.v2.BatchWriteSpansRequest
	1,  // 9: fixtures.TraceExpectFixture.self_observability_metrics:type_name -> fixtures.SelfObservabilityMetric
	10, // [10:10] is the sub-list for method output_type
	10, // [10:10] is the sub-list for method input_type
	10, // [10:10] is the sub-list for extension type_name
	10, // [10:10] is the sub-list for extension extendee
	0,  // [0:10] is the sub-list for field type_name
}

func init() { file_fixtures_proto_init() }
func file_fixtures_proto_init() {
	if File_fixtures_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_fixtures_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*MetricExpectFixture); i {
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
		file_fixtures_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SelfObservabilityMetric); i {
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
		file_fixtures_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*LogExpectFixture); i {
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
		file_fixtures_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TraceExpectFixture); i {
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
			RawDescriptor: file_fixtures_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_fixtures_proto_goTypes,
		DependencyIndexes: file_fixtures_proto_depIdxs,
		MessageInfos:      file_fixtures_proto_msgTypes,
	}.Build()
	File_fixtures_proto = out.File
	file_fixtures_proto_rawDesc = nil
	file_fixtures_proto_goTypes = nil
	file_fixtures_proto_depIdxs = nil
}
