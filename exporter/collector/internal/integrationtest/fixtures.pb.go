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
// 	protoc-gen-go v1.27.1
// 	protoc        v3.12.4
// source: fixtures.proto

package integrationtest

import (
	v3 "google.golang.org/genproto/googleapis/monitoring/v3"
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

	CreateTimeSeriesRequests        []*v3.CreateTimeSeriesRequest       `protobuf:"bytes,1,rep,name=create_time_series_requests,json=createTimeSeriesRequests,proto3" json:"create_time_series_requests,omitempty"`
	CreateMetricDescriptorRequests  []*v3.CreateMetricDescriptorRequest `protobuf:"bytes,2,rep,name=create_metric_descriptor_requests,json=createMetricDescriptorRequests,proto3" json:"create_metric_descriptor_requests,omitempty"`
	CreateServiceTimeSeriesRequests []*v3.CreateTimeSeriesRequest       `protobuf:"bytes,3,rep,name=create_service_time_series_requests,json=createServiceTimeSeriesRequests,proto3" json:"create_service_time_series_requests,omitempty"`
	SelfObservabilityMetrics        []*SelfObservabilityMetric          `protobuf:"bytes,4,rep,name=self_observability_metrics,json=selfObservabilityMetrics,proto3" json:"self_observability_metrics,omitempty"`
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

func (x *MetricExpectFixture) GetCreateTimeSeriesRequests() []*v3.CreateTimeSeriesRequest {
	if x != nil {
		return x.CreateTimeSeriesRequests
	}
	return nil
}

func (x *MetricExpectFixture) GetCreateMetricDescriptorRequests() []*v3.CreateMetricDescriptorRequest {
	if x != nil {
		return x.CreateMetricDescriptorRequests
	}
	return nil
}

func (x *MetricExpectFixture) GetCreateServiceTimeSeriesRequests() []*v3.CreateTimeSeriesRequest {
	if x != nil {
		return x.CreateServiceTimeSeriesRequests
	}
	return nil
}

func (x *MetricExpectFixture) GetSelfObservabilityMetrics() []*SelfObservabilityMetric {
	if x != nil {
		return x.SelfObservabilityMetrics
	}
	return nil
}

type SelfObservabilityMetric struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// metric name
	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	// stringified value
	Val string `protobuf:"bytes,2,opt,name=val,proto3" json:"val,omitempty"`
	// stringified labels
	Labels map[string]string `protobuf:"bytes,3,rep,name=labels,proto3" json:"labels,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
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

func (x *SelfObservabilityMetric) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *SelfObservabilityMetric) GetVal() string {
	if x != nil {
		return x.Val
	}
	return ""
}

func (x *SelfObservabilityMetric) GetLabels() map[string]string {
	if x != nil {
		return x.Labels
	}
	return nil
}

var File_fixtures_proto protoreflect.FileDescriptor

var file_fixtures_proto_rawDesc = []byte{
	0x0a, 0x0e, 0x66, 0x69, 0x78, 0x74, 0x75, 0x72, 0x65, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x12, 0x08, 0x66, 0x69, 0x78, 0x74, 0x75, 0x72, 0x65, 0x73, 0x1a, 0x14, 0x6d, 0x65, 0x74, 0x72,
	0x69, 0x63, 0x5f, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x22, 0xe1, 0x03, 0x0a, 0x13, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x45, 0x78, 0x70, 0x65, 0x63,
	0x74, 0x46, 0x69, 0x78, 0x74, 0x75, 0x72, 0x65, 0x12, 0x6c, 0x0a, 0x1b, 0x63, 0x72, 0x65, 0x61,
	0x74, 0x65, 0x5f, 0x74, 0x69, 0x6d, 0x65, 0x5f, 0x73, 0x65, 0x72, 0x69, 0x65, 0x73, 0x5f, 0x72,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x2d, 0x2e,
	0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x6d, 0x6f, 0x6e, 0x69, 0x74, 0x6f, 0x72, 0x69, 0x6e,
	0x67, 0x2e, 0x76, 0x33, 0x2e, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x54, 0x69, 0x6d, 0x65, 0x53,
	0x65, 0x72, 0x69, 0x65, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x52, 0x18, 0x63, 0x72,
	0x65, 0x61, 0x74, 0x65, 0x54, 0x69, 0x6d, 0x65, 0x53, 0x65, 0x72, 0x69, 0x65, 0x73, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x73, 0x12, 0x7e, 0x0a, 0x21, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65,
	0x5f, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x5f, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74,
	0x6f, 0x72, 0x5f, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28,
	0x0b, 0x32, 0x33, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x6d, 0x6f, 0x6e, 0x69, 0x74,
	0x6f, 0x72, 0x69, 0x6e, 0x67, 0x2e, 0x76, 0x33, 0x2e, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x4d,
	0x65, 0x74, 0x72, 0x69, 0x63, 0x44, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x6f, 0x72, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x52, 0x1e, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x4d, 0x65,
	0x74, 0x72, 0x69, 0x63, 0x44, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x6f, 0x72, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x73, 0x12, 0x7b, 0x0a, 0x23, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65,
	0x5f, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x5f, 0x74, 0x69, 0x6d, 0x65, 0x5f, 0x73, 0x65,
	0x72, 0x69, 0x65, 0x73, 0x5f, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x73, 0x18, 0x03, 0x20,
	0x03, 0x28, 0x0b, 0x32, 0x2d, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x6d, 0x6f, 0x6e,
	0x69, 0x74, 0x6f, 0x72, 0x69, 0x6e, 0x67, 0x2e, 0x76, 0x33, 0x2e, 0x43, 0x72, 0x65, 0x61, 0x74,
	0x65, 0x54, 0x69, 0x6d, 0x65, 0x53, 0x65, 0x72, 0x69, 0x65, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x52, 0x1f, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63,
	0x65, 0x54, 0x69, 0x6d, 0x65, 0x53, 0x65, 0x72, 0x69, 0x65, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x73, 0x12, 0x5f, 0x0a, 0x1a, 0x73, 0x65, 0x6c, 0x66, 0x5f, 0x6f, 0x62, 0x73, 0x65,
	0x72, 0x76, 0x61, 0x62, 0x69, 0x6c, 0x69, 0x74, 0x79, 0x5f, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63,
	0x73, 0x18, 0x04, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x21, 0x2e, 0x66, 0x69, 0x78, 0x74, 0x75, 0x72,
	0x65, 0x73, 0x2e, 0x53, 0x65, 0x6c, 0x66, 0x4f, 0x62, 0x73, 0x65, 0x72, 0x76, 0x61, 0x62, 0x69,
	0x6c, 0x69, 0x74, 0x79, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x52, 0x18, 0x73, 0x65, 0x6c, 0x66,
	0x4f, 0x62, 0x73, 0x65, 0x72, 0x76, 0x61, 0x62, 0x69, 0x6c, 0x69, 0x74, 0x79, 0x4d, 0x65, 0x74,
	0x72, 0x69, 0x63, 0x73, 0x22, 0xc1, 0x01, 0x0a, 0x17, 0x53, 0x65, 0x6c, 0x66, 0x4f, 0x62, 0x73,
	0x65, 0x72, 0x76, 0x61, 0x62, 0x69, 0x6c, 0x69, 0x74, 0x79, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63,
	0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04,
	0x6e, 0x61, 0x6d, 0x65, 0x12, 0x10, 0x0a, 0x03, 0x76, 0x61, 0x6c, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x03, 0x76, 0x61, 0x6c, 0x12, 0x45, 0x0a, 0x06, 0x6c, 0x61, 0x62, 0x65, 0x6c, 0x73,
	0x18, 0x03, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x2d, 0x2e, 0x66, 0x69, 0x78, 0x74, 0x75, 0x72, 0x65,
	0x73, 0x2e, 0x53, 0x65, 0x6c, 0x66, 0x4f, 0x62, 0x73, 0x65, 0x72, 0x76, 0x61, 0x62, 0x69, 0x6c,
	0x69, 0x74, 0x79, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x2e, 0x4c, 0x61, 0x62, 0x65, 0x6c, 0x73,
	0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x06, 0x6c, 0x61, 0x62, 0x65, 0x6c, 0x73, 0x1a, 0x39, 0x0a,
	0x0b, 0x4c, 0x61, 0x62, 0x65, 0x6c, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03,
	0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x14,
	0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x76,
	0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x42, 0x68, 0x5a, 0x66, 0x67, 0x69, 0x74, 0x68,
	0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x47, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x43, 0x6c, 0x6f,
	0x75, 0x64, 0x50, 0x6c, 0x61, 0x74, 0x66, 0x6f, 0x72, 0x6d, 0x2f, 0x6f, 0x70, 0x65, 0x6e, 0x74,
	0x65, 0x6c, 0x65, 0x6d, 0x65, 0x74, 0x72, 0x79, 0x2d, 0x6f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x73, 0x2d, 0x67, 0x6f, 0x2f, 0x65, 0x78, 0x70, 0x6f, 0x72, 0x74, 0x65, 0x72, 0x2f,
	0x63, 0x6f, 0x6c, 0x6c, 0x65, 0x63, 0x74, 0x6f, 0x72, 0x2f, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e,
	0x61, 0x6c, 0x2f, 0x69, 0x6e, 0x74, 0x65, 0x67, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x74, 0x65,
	0x73, 0x74, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
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

var file_fixtures_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_fixtures_proto_goTypes = []interface{}{
	(*MetricExpectFixture)(nil),              // 0: fixtures.MetricExpectFixture
	(*SelfObservabilityMetric)(nil),          // 1: fixtures.SelfObservabilityMetric
	nil,                                      // 2: fixtures.SelfObservabilityMetric.LabelsEntry
	(*v3.CreateTimeSeriesRequest)(nil),       // 3: google.monitoring.v3.CreateTimeSeriesRequest
	(*v3.CreateMetricDescriptorRequest)(nil), // 4: google.monitoring.v3.CreateMetricDescriptorRequest
}
var file_fixtures_proto_depIdxs = []int32{
	3, // 0: fixtures.MetricExpectFixture.create_time_series_requests:type_name -> google.monitoring.v3.CreateTimeSeriesRequest
	4, // 1: fixtures.MetricExpectFixture.create_metric_descriptor_requests:type_name -> google.monitoring.v3.CreateMetricDescriptorRequest
	3, // 2: fixtures.MetricExpectFixture.create_service_time_series_requests:type_name -> google.monitoring.v3.CreateTimeSeriesRequest
	1, // 3: fixtures.MetricExpectFixture.self_observability_metrics:type_name -> fixtures.SelfObservabilityMetric
	2, // 4: fixtures.SelfObservabilityMetric.labels:type_name -> fixtures.SelfObservabilityMetric.LabelsEntry
	5, // [5:5] is the sub-list for method output_type
	5, // [5:5] is the sub-list for method input_type
	5, // [5:5] is the sub-list for extension type_name
	5, // [5:5] is the sub-list for extension extendee
	0, // [0:5] is the sub-list for field type_name
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
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_fixtures_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   3,
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
