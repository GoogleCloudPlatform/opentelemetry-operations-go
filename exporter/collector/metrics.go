// Copyright 2022 Google LLC
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

// This file contains the rewritten googlecloud metrics exporter which no longer takes
// dependency on the OpenCensus stackdriver exporter.

package collector

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net/url"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/genproto/googleapis/api/distribution"
	"google.golang.org/genproto/googleapis/api/label"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/otel/attribute"
	metricapi "go.opentelemetry.io/otel/metric"

	"github.com/fsnotify/fsnotify"
	"github.com/tidwall/wal"
	"go.uber.org/zap"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/internal/datapointstorage"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/internal/normalization"
	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping"
)

// self-observability reporting meters/tracers/loggers.
type selfObservability struct {
	// Logger to use for this exporter.
	log           *zap.Logger
	meterProvider metricapi.MeterProvider
}

// MetricsExporter is the GCM exporter that uses pdata directly.
type MetricsExporter struct {
	mapper metricMapper
	obs    selfObservability
	client monitoringClient
	// self-observability metrics
	pointsExportedCounter metricapi.Int64Counter
	// Only used for testing purposes in lieu of initializing a fake client
	exportFunc func(context.Context, *monitoringpb.CreateTimeSeriesRequest) error
	// A channel that receives metric descriptor and sends them to GCM once
	metricDescriptorC chan *monitoringpb.CreateMetricDescriptorRequest
	// write ahead log handles exporter retries in-order to handle network outages
	wal *exporterWAL
	// mdCache tracks the metric descriptors that have already been sent to GCM
	mdCache map[string]*monitoringpb.CreateMetricDescriptorRequest
	// shutdownC is a channel for signaling a graceful shutdown
	shutdownC chan struct{}
	// requestOpts applies options to the context for requests, such as additional headers.
	requestOpts []func(*context.Context, requestInfo)
	cfg         Config
	// goroutines tracks the currently running child tasks
	goroutines sync.WaitGroup
	timeout    time.Duration
}

type exporterWAL struct {
	*wal.Log
	// the full path of the WAL (user-configured directory + "gcp_metrics_wal")
	path       string
	maxBackoff time.Duration
	mutex      sync.Mutex
}

// requestInfo is meant to abstract info from CreateMetricsDescriptorRequests and
// CreateTimeSeriesRequests that is shared by requestOpts functions.
type requestInfo struct {
	projectName string
}

// metricMapper is the part that transforms metrics. Separate from MetricsExporter since it has
// all pure functions.
type metricMapper struct {
	normalizer                  normalization.Normalizer
	obs                         selfObservability
	exemplarAttachmentDropCount metricapi.Int64Counter
	cfg                         Config
}

// Constants we use when translating summary metrics into GCP.
const (
	SummaryCountPrefix = "_count"
	SummarySumSuffix   = "_sum"
	scopeName          = "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
)

const (
	// The number of timeserieses to send to GCM in a single request. This
	// is a hard limit in the GCM API, so we never want to exceed 200.
	sendBatchSize = 200

	// The default amount of time to retry a data point on network outage when
	// WAL is enabled before discarding. Users can override by setting MetricConfig.WALConfig.MaxBackoff.
	defaultWalMaxBackoff = time.Duration(3600 * time.Second)
)

const (
	// The specific unit that needs to be present in an integer-valued metric so
	// that it can be treated as a boolean.
	specialIntToBoolUnit = "{gcp.BOOL}"
)

type labels map[string]string

// monitoringClient is the subset of monitoring.MetricClient this exporter uses,
// and allows us to mock the implementation for testing.
type monitoringClient interface {
	CreateTimeSeries(ctx context.Context, req *monitoringpb.CreateTimeSeriesRequest, opts ...gax.CallOption) error
	CreateServiceTimeSeries(ctx context.Context, req *monitoringpb.CreateTimeSeriesRequest, opts ...gax.CallOption) error
	Close() error
	CreateMetricDescriptor(ctx context.Context, req *monitoringpb.CreateMetricDescriptorRequest, opts ...gax.CallOption) (*metricpb.MetricDescriptor, error)
}

func (me *MetricsExporter) Shutdown(ctx context.Context) error {
	// TODO: pass ctx to goroutines so that we can use its deadline
	close(me.shutdownC)
	c := make(chan struct{})
	go func() {
		// Wait until all goroutines are done
		me.goroutines.Wait()
		close(c)
	}()
	select {
	case <-ctx.Done():
		me.obs.log.Error("Error waiting for async tasks to finish.", zap.Error(ctx.Err()))
	case <-c:
	}
	if me.client != nil {
		return me.client.Close()
	}
	return nil
}

func NewGoogleCloudMetricsExporter(
	ctx context.Context,
	cfg Config,
	set exporter.Settings,
	timeout time.Duration,
) (*MetricsExporter, error) {
	SetUserAgent(&cfg, set.BuildInfo)
	meter := set.TelemetrySettings.MeterProvider.Meter(scopeName, metricapi.WithInstrumentationVersion(Version()))
	pointsExportedCounter, err := meter.Int64Counter(
		"googlecloudmonitoring/point_count",
		metricapi.WithDescription("Count of metric points written to Cloud Monitoring."),
		metricapi.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}
	exemplarAttachmentDropCount, err := meter.Int64Counter(
		"googlecloudmonitoring/exemplar_attachments_dropped",
		metricapi.WithDescription("Count of exemplar attachments dropped."),
		metricapi.WithUnit("{attachments}"),
	)
	if err != nil {
		return nil, err
	}

	obs := selfObservability{
		log:           set.TelemetrySettings.Logger,
		meterProvider: set.TelemetrySettings.MeterProvider,
	}
	normalizer := normalization.NewDisabledNormalizer()
	mExp := &MetricsExporter{
		cfg: cfg,
		obs: obs,
		mapper: metricMapper{
			obs:                         obs,
			cfg:                         cfg,
			normalizer:                  normalizer,
			exemplarAttachmentDropCount: exemplarAttachmentDropCount,
		},
		// We create a buffered channel for metric descriptors.
		// MetricDescritpors are asychronously sent and optimistic.
		// We only get Unit/Description/Display name from them, so it's ok
		// to drop / conserve resources for sending timeseries.
		metricDescriptorC:     make(chan *monitoringpb.CreateMetricDescriptorRequest, cfg.MetricConfig.CreateMetricDescriptorBufferSize),
		mdCache:               make(map[string]*monitoringpb.CreateMetricDescriptorRequest),
		shutdownC:             make(chan struct{}),
		timeout:               timeout,
		pointsExportedCounter: pointsExportedCounter,
	}
	mExp.exportFunc = mExp.exportToTimeSeries

	mExp.requestOpts = make([]func(*context.Context, requestInfo), 0)
	if cfg.DestinationProjectQuota {
		mExp.requestOpts = append(mExp.requestOpts, func(ctx *context.Context, ri requestInfo) {
			*ctx = metadata.NewOutgoingContext(*ctx, metadata.New(map[string]string{"x-goog-user-project": strings.TrimPrefix(ri.projectName, "projects/")}))
		})
	}

	return mExp, nil
}

func (me *MetricsExporter) Start(ctx context.Context, _ component.Host) error {
	me.shutdownC = make(chan struct{})
	if me.cfg.MetricConfig.CumulativeNormalization {
		me.mapper.normalizer = normalization.NewStandardNormalizer(me.shutdownC, me.obs.log)
	}
	clientOpts, err := generateClientOptions(ctx, &me.cfg.MetricConfig.ClientConfig, &me.cfg, monitoring.DefaultAuthScopes(), me.obs.meterProvider)
	if err != nil {
		return err
	}

	client, err := monitoring.NewMetricClient(ctx, clientOpts...)
	if err != nil {
		return err
	}

	if me.cfg.MetricConfig.ClientConfig.Compression == gzip.Name {
		client.CallOptions.CreateMetricDescriptor = append(client.CallOptions.CreateMetricDescriptor,
			gax.WithGRPCOptions(grpc.UseCompressor(gzip.Name)))
		client.CallOptions.CreateTimeSeries = append(client.CallOptions.CreateTimeSeries,
			gax.WithGRPCOptions(grpc.UseCompressor(gzip.Name)))
		client.CallOptions.CreateServiceTimeSeries = append(client.CallOptions.CreateServiceTimeSeries,
			gax.WithGRPCOptions(grpc.UseCompressor(gzip.Name)))
	}
	me.client = client
	if me.cfg.MetricConfig.WALConfig != nil {
		_, _, err = me.setupWAL()
		if err != nil {
			return err
		}
		// start WAL popper routine
		me.goroutines.Add(1)
		go me.runWALReadAndExportLoop(ctx)
	}

	// Fire up the metric descriptor exporter.
	me.goroutines.Add(1)
	go me.exportMetricDescriptorRunner()
	return nil
}

// setupWAL creates the WAL.
// This function is also used to re-sync after writes, so it closes the existing WAL if present.
// It returns the FirstIndex, LastIndex, and any error.
func (me *MetricsExporter) setupWAL() (uint64, uint64, error) {
	err := me.closeWAL()
	if err != nil {
		return 0, 0, err
	}

	if me.wal == nil {
		me.wal = &exporterWAL{}
	}

	walPath := filepath.Join(me.cfg.MetricConfig.WALConfig.Directory, "gcp_metrics_wal")
	me.wal.path = walPath
	metricWal, err := wal.Open(walPath, &wal.Options{LogFormat: 1})
	if err != nil {
		return 0, 0, err
	}
	me.wal.Log = metricWal

	// default to 1 hour exponential backoff
	me.wal.maxBackoff = defaultWalMaxBackoff
	if me.cfg.MetricConfig.WALConfig.MaxBackoff != 0 {
		me.wal.maxBackoff = me.cfg.MetricConfig.WALConfig.MaxBackoff
	}

	// sync existing WAL indices
	rIndex, err := me.wal.FirstIndex()
	if err != nil {
		return 0, 0, err
	}

	wIndex, err := me.wal.LastIndex()
	if err != nil {
		return 0, 0, err
	}

	return rIndex, wIndex, nil
}

func (me *MetricsExporter) closeWAL() error {
	if me.wal != nil && me.wal.Log != nil {
		err := me.wal.Log.Close()
		me.wal.Log = nil
		return err
	}
	return nil
}

// PushMetrics calls pushes pdata metrics to GCM, creating metric descriptors if necessary.
func (me *MetricsExporter) PushMetrics(ctx context.Context, m pmetric.Metrics) error {
	if me.client == nil {
		return errors.New("not started")
	}
	if me.wal != nil {
		me.wal.mutex.Lock()
		defer me.wal.mutex.Unlock()
	}
	// map from project -> []timeseries. This groups timeseries by the project
	// they need to be sent to. Each project's timeseries are sent in a
	// separate request later.
	pendingTimeSeries := map[string][]*monitoringpb.TimeSeries{}

	// add extra metrics from the ExtraMetrics() extension point
	if me.cfg.MetricConfig.ExtraMetrics != nil {
		me.cfg.MetricConfig.ExtraMetrics(m)
	}
	rms := m.ResourceMetrics()

	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		monitoredResource := me.cfg.MetricConfig.MapMonitoredResource(rm.Resource())
		extraResourceLabels := attributesToLabels(filterAttributes(rm.Resource().Attributes(), me.cfg.MetricConfig.ServiceResourceLabels, me.cfg.MetricConfig.ResourceFilters))
		projectID := me.cfg.ProjectID
		// override project ID with gcp.project.id, if present
		if projectFromResource, found := rm.Resource().Attributes().Get(resourcemapping.ProjectIDAttributeKey); found {
			projectID = projectFromResource.AsString()
		}
		sms := rm.ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)

			instrumentationScopeLabels := me.mapper.instrumentationScopeToLabels(sm.Scope())
			metricLabels := mergeLabels(nil, instrumentationScopeLabels, extraResourceLabels)

			mes := sm.Metrics()
			for k := 0; k < mes.Len(); k++ {
				metric := mes.At(k)
				pendingTimeSeries[projectID] = append(pendingTimeSeries[projectID], me.mapper.metricToTimeSeries(monitoredResource, metricLabels, metric, projectID)...)

				// We only send metric descriptors if we're configured *and* we're not sending service timeseries.
				if me.cfg.MetricConfig.SkipCreateMetricDescriptor || me.cfg.MetricConfig.CreateServiceTimeSeries {
					continue
				}

				for _, md := range me.mapper.metricDescriptor(metric, metricLabels) {
					if md == nil {
						continue
					}
					req := &monitoringpb.CreateMetricDescriptorRequest{
						Name:             projectName(projectID),
						MetricDescriptor: md,
					}
					select {
					case me.metricDescriptorC <- req:
					default:
						// Ignore drops, we'll catch descriptor next time around.
					}
				}
			}
		}
	}

	var errs []error
	// timeseries for each project are batched and exported separately
	for projectID, projectTS := range pendingTimeSeries {
		// Batch and export
		for len(projectTS) > 0 {
			var sendSize int
			if len(projectTS) < sendBatchSize {
				sendSize = len(projectTS)
			} else {
				sendSize = sendBatchSize
			}

			var ts []*monitoringpb.TimeSeries
			ts, projectTS = projectTS[:sendSize], projectTS[sendSize:]

			req := &monitoringpb.CreateTimeSeriesRequest{
				Name:       projectName(projectID),
				TimeSeries: ts,
			}

			if me.wal != nil {
				// push request onto the WAL
				bytes, err := proto.Marshal(req)
				if err != nil {
					errs = append(errs, fmt.Errorf("failed to marshal protobuf to bytes: %+v", err))
					continue
				}

				writeIndex, err := me.wal.LastIndex()
				if err != nil {
					errs = append(errs, fmt.Errorf("failed to get LastIndex of WAL: %+v", err))
					continue
				}

				err = me.wal.Write(writeIndex+1, bytes)
				if err != nil {
					errs = append(errs, fmt.Errorf("failed to write to WAL: %+v", err))
					continue
				}
			} else {
				// otherwise export directly
				errs = append(errs, me.export(ctx, req))
			}
		}
	}
	return errors.Join(errs...)
}

// exportToTimeSeries is the default exporting call to GCM.
// Broken into its own function for unit testing.
func (me *MetricsExporter) exportToTimeSeries(ctx context.Context, req *monitoringpb.CreateTimeSeriesRequest) error {
	var err error
	if me.cfg.MetricConfig.CreateServiceTimeSeries {
		err = me.createServiceTimeSeries(ctx, req)
	} else {
		err = me.createTimeSeries(ctx, req)
	}
	return err
}

// export sends a CreateTimeSeriesRequest to GCM and reports failed/successful points based on the response.
func (me *MetricsExporter) export(ctx context.Context, req *monitoringpb.CreateTimeSeriesRequest) error {
	// if this is an empty request, skip it
	// empty requests are used by the WAL to signal the end of pending data
	if isEmptyReq(req) {
		return nil
	}

	err := me.exportFunc(ctx, req)
	s := status.Convert(err)

	succeededPoints := len(req.TimeSeries)
	failedPoints := 0
	for _, detail := range s.Details() {
		if summary, ok := detail.(*monitoringpb.CreateTimeSeriesSummary); ok {
			failedPoints = int(summary.TotalPointCount - summary.SuccessPointCount)
			succeededPoints = int(summary.SuccessPointCount)
		}
	}

	// always record the number of successful points
	me.pointsExportedCounter.Add(
		ctx,
		int64(succeededPoints),
		metricapi.WithAttributes(attribute.String("status", "OK")),
	)
	if failedPoints > 0 {
		st := statusCodeToString(s)
		me.pointsExportedCounter.Add(
			ctx,
			int64(failedPoints),
			metricapi.WithAttributes(attribute.String("status", st)),
		)
	}
	if err != nil {
		return fmt.Errorf("failed to export metrics to %v: %w", req.Name, err)
	}
	return nil
}

func statusCodeToString(s *status.Status) string {
	// see https://github.com/grpc/grpc/blob/master/doc/statuscodes.md
	switch c := s.Code(); c {
	case codes.OK:
		return "OK"
	case codes.Canceled:
		return "CANCELLED"
	case codes.Unknown:
		return "UNKNOWN"
	case codes.InvalidArgument:
		return "INVALID_ARGUMENT"
	case codes.DeadlineExceeded:
		return "DEADLINE_EXCEEDED"
	case codes.NotFound:
		return "NOT_FOUND"
	case codes.AlreadyExists:
		return "ALREADY_EXISTS"
	case codes.PermissionDenied:
		return "PERMISSION_DENIED"
	case codes.ResourceExhausted:
		return "RESOURCE_EXHAUSTED"
	case codes.FailedPrecondition:
		return "FAILED_PRECONDITION"
	case codes.Aborted:
		return "ABORTED"
	case codes.OutOfRange:
		return "OUT_OF_RANGE"
	case codes.Unimplemented:
		return "UNIMPLEMENTED"
	case codes.Internal:
		return "INTERNAL"
	case codes.Unavailable:
		return "UNAVAILABLE"
	case codes.DataLoss:
		return "DATA_LOSS"
	case codes.Unauthenticated:
		return "UNAUTHENTICATED"
	default:
		return "CODE_" + strconv.FormatInt(int64(c), 10)
	}
}

// readWALAndExport pops the next CreateTimeSeriesRequest from the WAL and tries exporting it.
// If the export is successful (or fails for a non-retryable error), the read index is incremented
// so the next entry in the WAL can be read by a subsequent call to readWALAndExport().
// If the export fails for a (retryable) network error, it will keep trying to export the same entry
// until success or the backoff max is reached.
func (me *MetricsExporter) readWALAndExport(ctx context.Context) error {
	me.wal.mutex.Lock()
	defer me.wal.mutex.Unlock()

	// close and reopen the WAL to sync indices
	readIndex, writeIndex, err := me.setupWAL()
	if err != nil {
		return err
	}

	bytes, err := me.wal.Read(readIndex)
	if err == nil {
		req := new(monitoringpb.CreateTimeSeriesRequest)
		if err = proto.Unmarshal(bytes, req); err != nil {
			return err
		}

		// on network failures, retry exponentially a max of 11 times (2^12s > 48 hours, older than allowed by GCM)
		// or until user-configured max backoff is hit.
		backoff := 0
		for i := 0; i < 12; i++ {
			err = me.export(ctx, req)
			if err != nil {
				me.obs.log.Warn(fmt.Sprintf("error exporting to GCM: %+v", err))
			}
			// retry at same read index if retryable (network) error
			if isNotRecoverable(err) {
				break
			}
			me.obs.log.Error("retryable error, retrying request")
			backoff = 1 << i
			if time.Duration(backoff)*time.Second >= me.wal.maxBackoff {
				break
			}
			time.Sleep(time.Duration(backoff) * time.Second)
		}

		// If we are at the last index, and this last index is not an empty request
		// (we use empty requests to fill out the end of a log, and if we didn't check for them
		// this would loop constantly adding empty requests onto the end)
		if readIndex == writeIndex && !isEmptyReq(req) {
			// This indicates that we are trying to truncate the last item in the WAL.
			// If that is the case, write an empty request so we can truncate the last real request
			// (the WAL library requires at least 1 entry).
			// Doing so prevents double-exporting in the event of a collector restart.
			emptyReq := &monitoringpb.CreateTimeSeriesRequest{}
			bytes, bytesErr := proto.Marshal(emptyReq)
			if bytesErr != nil {
				return bytesErr
			}

			writeIndex++
			err = me.wal.Write(writeIndex, bytes)
			if err != nil {
				return err
			}
		}

		// Truncate if readIndex < writeIndex.
		// This only happens if there are more entries in the WAL
		// OR, we are at the last real entry and added an "empty" entry above, in which we also increment writeIndex.
		// otherwise, we've reached the end of the WAL and should be at an empty entry, which the export drops.
		// If that's the case, and we try to truncate (ie, move readIndex+1), the library returns ErrOutOfRange.
		if readIndex >= writeIndex {
			// wal.ErrNotFound is used by wal.Read() to indicate the end of the WAL, but
			// the wal library doesn't know about our hackery around empty entries.
			// So it's used by us to indicate the same.
			return wal.ErrNotFound
		}
		err = me.wal.TruncateFront(readIndex + 1)
		if err != nil {
			return err
		}
	}
	return err
}

// watchWAL watches the WAL directory for a write then returns to the
// continuallyPopWAL() loop.
func (me *MetricsExporter) watchWALFile(ctx context.Context) error {
	me.goroutines.Add(1)
	defer me.goroutines.Done()
	walWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	err = walWatcher.Add(me.wal.path)
	if err != nil {
		return err
	}
	watchCh := make(chan error)
	var wErr error
	go func() {
		defer func() {
			watchCh <- wErr
			close(watchCh)
			walWatcher.Close()
		}()

		select {
		case <-me.shutdownC:
			return
		case <-ctx.Done():
			wErr = ctx.Err()
			return

		case event, ok := <-walWatcher.Events:
			if !ok {
				return
			}
			switch event.Op {
			case fsnotify.Remove:
				wErr = fmt.Errorf("WAL file deleted")
			case fsnotify.Rename:
				wErr = fmt.Errorf("WAL file renamed")
			case fsnotify.Write:
				wErr = nil
			}

		case watchErr, ok := <-walWatcher.Errors:
			if ok {
				wErr = watchErr
			}
		}
	}()
	err = <-watchCh
	return err
}

func (me *MetricsExporter) runWALReadAndExportLoop(ctx context.Context) {
	defer me.goroutines.Done()
	defer func() {
		if err := me.wal.Close(); err != nil {
			me.obs.log.Error(fmt.Sprintf("error closing WAL: %+v\n", err))
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-me.shutdownC:
			// do one last final read/export then return
			// otherwise the runner goroutine could leave some hanging metrics unexported
			for {
				err := me.readWALAndExport(ctx)
				if err != nil {
					if !errors.Is(err, wal.ErrOutOfRange) {
						me.obs.log.Error(fmt.Sprintf("error flushing remaining WAL entries: %+v", err))
					}
					break
				}
			}
			return
		default:
			err := me.readWALAndExport(ctx)
			if err == nil {
				continue
			}
			// ErrNotFound from wal.Read() means the index is either 0 or out of
			// bounds (indicating we're probably at the end of the WAL). That error
			// will trigger a file watch for new writes (below this). For other
			// errors, fail.
			// ErrNotFound can be expected occasionally if we've reached the end of
			// the WAL, so don't bother logging those.
			if !errors.Is(err, wal.ErrNotFound) {
				me.obs.log.Error(fmt.Sprintf("error reading WAL and exporting: %+v", err))
			}

			// Must have been ErrNotFound, start a file watch and block waiting for updates.
			if err = me.watchWALFile(ctx); err != nil {
				me.obs.log.Error(fmt.Sprintf("error watching WAL and exporting: %+v", err))
			}
		}
	}
}

// Reads metric descriptors from the md channel, and reports them (once) to GCM.
func (me *MetricsExporter) exportMetricDescriptorRunner() {
	defer me.goroutines.Done()

	// We iterate over all metric descritpors until the channel is closed.
	// Note: if we get terminated, this will still attempt to export all descriptors
	// prior to shutdown.
	for {
		select {
		case <-me.shutdownC:
			for {
				// We are shutting down. Publish all the pending
				// items on the channel before we stop.
				select {
				case md := <-me.metricDescriptorC:
					me.exportMetricDescriptor(md)
				default:
					// Return and continue graceful shutdown.
					return
				}
			}

		case md := <-me.metricDescriptorC:
			me.exportMetricDescriptor(md)
		}
	}
}

func projectName(projectID string) string {
	return fmt.Sprintf("projects/%s", projectID)
}

// isNotRecoverable returns true if the error is permanent.
func isNotRecoverable(err error) bool {
	s := status.Convert(err)
	return !(s.Code() == codes.DeadlineExceeded || s.Code() == codes.Unavailable)
}

// Helper method to send metric descriptors to GCM.
func (me *MetricsExporter) exportMetricDescriptor(req *monitoringpb.CreateMetricDescriptorRequest) {
	cacheKey := fmt.Sprintf("%s/%s", req.Name, req.MetricDescriptor.Type)
	if _, exists := me.mdCache[cacheKey]; exists {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), me.timeout)
	defer cancel()

	for _, opt := range me.requestOpts {
		opt(&ctx, requestInfo{projectName: req.Name})
	}
	_, err := me.client.CreateMetricDescriptor(ctx, req)
	if err != nil {
		if isNotRecoverable(err) {
			// cache if the error is non-recoverable
			me.mdCache[cacheKey] = req
		}
		// TODO: Log-once on error, per metric descriptor?
		me.obs.log.Error("Unable to send metric descriptor.", zap.Error(err), zap.Any("metric_descriptor", req.MetricDescriptor))
		return
	}

	// cache if we are successful
	me.mdCache[cacheKey] = req
}

// Sends a user-custom-metric timeseries.
func (me *MetricsExporter) createTimeSeries(ctx context.Context, req *monitoringpb.CreateTimeSeriesRequest) error {
	ctx, cancel := context.WithTimeout(ctx, me.timeout)
	defer cancel()
	for _, opt := range me.requestOpts {
		opt(&ctx, requestInfo{projectName: req.Name})
	}
	return me.client.CreateTimeSeries(ctx, req)
}

// Sends a service timeseries.
func (me *MetricsExporter) createServiceTimeSeries(ctx context.Context, req *monitoringpb.CreateTimeSeriesRequest) error {
	ctx, cancel := context.WithTimeout(ctx, me.timeout)
	defer cancel()
	for _, opt := range me.requestOpts {
		opt(&ctx, requestInfo{projectName: req.Name})
	}
	return me.client.CreateServiceTimeSeries(ctx, req)
}

func (m *metricMapper) instrumentationScopeToLabels(is pcommon.InstrumentationScope) labels {
	isLabels := make(labels)
	if !m.cfg.MetricConfig.InstrumentationLibraryLabels {
		return isLabels
	}
	instrumentationSource := sanitizeUTF8(is.Name())
	if len(instrumentationSource) > 0 {
		isLabels["instrumentation_source"] = instrumentationSource
	}
	instrumentationVersion := sanitizeUTF8(is.Version())
	if len(instrumentationVersion) > 0 {
		isLabels["instrumentation_version"] = instrumentationVersion
	}
	return isLabels
}

func (m *metricMapper) metricToTimeSeries(
	resource *monitoredrespb.MonitoredResource,
	extraLabels labels,
	metric pmetric.Metric,
	projectID string,
) []*monitoringpb.TimeSeries {
	timeSeries := []*monitoringpb.TimeSeries{}

	switch metric.Type() {
	case pmetric.MetricTypeSum:
		sum := metric.Sum()
		points := sum.DataPoints()
		for i := 0; i < points.Len(); i++ {
			ts := m.sumPointToTimeSeries(resource, extraLabels, metric, sum, points.At(i))
			timeSeries = append(timeSeries, ts...)
		}
	case pmetric.MetricTypeGauge:
		gauge := metric.Gauge()
		points := gauge.DataPoints()
		for i := 0; i < points.Len(); i++ {
			ts := m.gaugePointToTimeSeries(resource, extraLabels, metric, gauge, points.At(i))
			timeSeries = append(timeSeries, ts...)
		}
	case pmetric.MetricTypeSummary:
		summary := metric.Summary()
		points := summary.DataPoints()
		for i := 0; i < points.Len(); i++ {
			ts := m.summaryPointToTimeSeries(resource, extraLabels, metric, summary, points.At(i))
			timeSeries = append(timeSeries, ts...)
		}
	case pmetric.MetricTypeHistogram:
		hist := metric.Histogram()
		points := hist.DataPoints()
		for i := 0; i < points.Len(); i++ {
			ts := m.histogramToTimeSeries(resource, extraLabels, metric, hist, points.At(i), projectID)
			timeSeries = append(timeSeries, ts...)
		}
	case pmetric.MetricTypeExponentialHistogram:
		eh := metric.ExponentialHistogram()
		points := eh.DataPoints()
		for i := 0; i < points.Len(); i++ {
			ts := m.exponentialHistogramToTimeSeries(resource, extraLabels, metric, eh, points.At(i), projectID)
			timeSeries = append(timeSeries, ts...)
		}
	default:
		m.obs.log.Error("Unsupported metric data type", zap.Any("data_type", metric.Type()))
	}

	return timeSeries
}

func (m *metricMapper) summaryPointToTimeSeries(
	resource *monitoredrespb.MonitoredResource,
	extraLabels labels,
	metric pmetric.Metric,
	_ pmetric.Summary,
	point pmetric.SummaryDataPoint,
) []*monitoringpb.TimeSeries {
	if point.Flags().NoRecordedValue() {
		// Drop points without a value.
		return nil
	}
	// Normalize the summary point.
	metricIdentifier := datapointstorage.Identifier(resource, extraLabels, metric, point.Attributes())
	keep := m.normalizer.NormalizeSummaryDataPoint(point, metricIdentifier)
	if !keep {
		return nil
	}
	sumType, countType, quantileType, err := m.summaryMetricTypes(metric)
	if err != nil {
		m.obs.log.Debug("Failed to get metric type (i.e. name) for summary metric. Dropping the metric.", zap.Error(err), zap.Any("metric", metric))
		return nil
	}
	startTime := timestamppb.New(point.StartTimestamp().AsTime())
	endTime := timestamppb.New(point.Timestamp().AsTime())
	result := []*monitoringpb.TimeSeries{
		{
			Resource:    resource,
			Unit:        metric.Unit(),
			Description: metric.Description(),
			MetricKind:  metricpb.MetricDescriptor_CUMULATIVE,
			ValueType:   metricpb.MetricDescriptor_DOUBLE,
			Points: []*monitoringpb.Point{{
				Interval: &monitoringpb.TimeInterval{
					StartTime: startTime,
					EndTime:   endTime,
				},
				Value: &monitoringpb.TypedValue{Value: &monitoringpb.TypedValue_DoubleValue{
					DoubleValue: point.Sum(),
				}},
			}},
			Metric: &metricpb.Metric{
				Type: sumType,
				Labels: mergeLabels(
					attributesToLabels(point.Attributes()),
					extraLabels,
				),
			},
		},
		{
			Resource:    resource,
			Unit:        metric.Unit(),
			Description: metric.Description(),
			MetricKind:  metricpb.MetricDescriptor_CUMULATIVE,
			ValueType:   metricpb.MetricDescriptor_DOUBLE,
			Points: []*monitoringpb.Point{{
				Interval: &monitoringpb.TimeInterval{
					StartTime: startTime,
					EndTime:   endTime,
				},
				Value: &monitoringpb.TypedValue{Value: &monitoringpb.TypedValue_DoubleValue{
					DoubleValue: float64(point.Count()),
				}},
			}},
			Metric: &metricpb.Metric{
				Type: countType,
				Labels: mergeLabels(
					attributesToLabels(point.Attributes()),
					extraLabels,
				),
			},
		},
	}
	quantiles := point.QuantileValues()
	for i := 0; i < quantiles.Len(); i++ {
		quantile := quantiles.At(i)
		pLabel := labels{
			"quantile": strconv.FormatFloat(quantile.Quantile(), 'f', -1, 64),
		}
		result = append(result, &monitoringpb.TimeSeries{
			Resource:    resource,
			Unit:        metric.Unit(),
			Description: metric.Description(),
			MetricKind:  metricpb.MetricDescriptor_GAUGE,
			ValueType:   metricpb.MetricDescriptor_DOUBLE,
			Points: []*monitoringpb.Point{{
				Interval: &monitoringpb.TimeInterval{
					EndTime: endTime,
				},
				Value: &monitoringpb.TypedValue{Value: &monitoringpb.TypedValue_DoubleValue{
					DoubleValue: quantile.Value(),
				}},
			}},
			Metric: &metricpb.Metric{
				Type: quantileType,
				Labels: mergeLabels(
					attributesToLabels(point.Attributes()),
					extraLabels,
					pLabel,
				),
			},
		})
	}
	return result
}

func (m *metricMapper) exemplar(ex pmetric.Exemplar, projectID string) *distribution.Distribution_Exemplar {
	ctx := context.TODO()
	attachments := []*anypb.Any{}
	// TODO: Look into still sending exemplars with no span.
	if traceID, spanID := ex.TraceID(), ex.SpanID(); !traceID.IsEmpty() && !spanID.IsEmpty() {
		sctx, err := anypb.New(&monitoringpb.SpanContext{
			// TODO - make sure project id is correct.
			SpanName: fmt.Sprintf("projects/%s/traces/%s/spans/%s", projectID, hex.EncodeToString(traceID[:]), hex.EncodeToString(spanID[:])),
		})
		if err == nil {
			attachments = append(attachments, sctx)
		} else {
			// This happens in the event of logic error (e.g. missing required fields).
			// As such we complaining loudly to fail our unit tests.
			m.exemplarAttachmentDropCount.Add(ctx, 1)
		}
	}
	if ex.FilteredAttributes().Len() > 0 {
		attr, err := anypb.New(&monitoringpb.DroppedLabels{
			Label: attributesToLabels(ex.FilteredAttributes()),
		})
		if err == nil {
			attachments = append(attachments, attr)
		} else {
			// This happens in the event of logic error (e.g. missing required fields).
			// As such we complaining loudly to fail our unit tests.
			m.exemplarAttachmentDropCount.Add(ctx, 1)
		}
	}
	var val float64
	switch ex.ValueType() {
	case pmetric.ExemplarValueTypeDouble:
		val = ex.DoubleValue()
	case pmetric.ExemplarValueTypeInt:
		val = float64(ex.IntValue())
	}
	return &distribution.Distribution_Exemplar{
		Value:       val,
		Timestamp:   timestamppb.New(ex.Timestamp().AsTime()),
		Attachments: attachments,
	}
}

func (m *metricMapper) exemplars(exs pmetric.ExemplarSlice, projectID string) []*distribution.Distribution_Exemplar {
	exemplars := make([]*distribution.Distribution_Exemplar, exs.Len())
	for i := 0; i < exs.Len(); i++ {
		exemplars[i] = m.exemplar(exs.At(i), projectID)
	}
	sort.Slice(exemplars, func(i, j int) bool {
		return exemplars[i].Value < exemplars[j].Value
	})

	return exemplars
}

// histogramPoint maps a histogram data point into a GCM point.
func (m *metricMapper) histogramPoint(point pmetric.HistogramDataPoint, projectID string) *monitoringpb.TypedValue {
	counts := make([]int64, point.BucketCounts().Len())
	var mean, deviation, prevBound float64

	for i := 0; i < point.BucketCounts().Len(); i++ {
		counts[i] = int64(point.BucketCounts().At(i))
	}

	if !math.IsNaN(point.Sum()) && point.Count() > 0 { // Avoid divide-by-zero
		mean = float64(point.Sum() / float64(point.Count()))
	}

	bounds := point.ExplicitBounds()
	if m.cfg.MetricConfig.EnableSumOfSquaredDeviation {
		// Calculate the sum of squared deviation.
		for i := 0; i < bounds.Len(); i++ {
			// Assume all points in the bucket occur at the middle of the bucket range
			middleOfBucket := (prevBound + bounds.At(i)) / 2
			deviation += float64(counts[i]) * (middleOfBucket - mean) * (middleOfBucket - mean)
			prevBound = bounds.At(i)
		}
		// The infinity bucket is an implicit +Inf bound after the list of explicit bounds.
		// Assume points in the infinity bucket are at the top of the previous bucket
		middleOfInfBucket := prevBound
		deviation += float64(counts[len(counts)-1]) * (middleOfInfBucket - mean) * (middleOfInfBucket - mean)
	}

	return &monitoringpb.TypedValue{
		Value: &monitoringpb.TypedValue_DistributionValue{
			DistributionValue: &distribution.Distribution{
				Count:                 int64(point.Count()),
				Mean:                  mean,
				BucketCounts:          counts,
				SumOfSquaredDeviation: deviation,
				BucketOptions: &distribution.Distribution_BucketOptions{
					Options: &distribution.Distribution_BucketOptions_ExplicitBuckets{
						ExplicitBuckets: &distribution.Distribution_BucketOptions_Explicit{
							Bounds: bounds.AsRaw(),
						},
					},
				},
				Exemplars: m.exemplars(point.Exemplars(), projectID),
			},
		},
	}
}

// Maps an exponential distribution into a GCM point.
func (m *metricMapper) exponentialHistogramPoint(point pmetric.ExponentialHistogramDataPoint, projectID string) *monitoringpb.TypedValue {
	// First calculate underflow bucket with all negatives + zeros.
	underflow := point.ZeroCount()
	negativeBuckets := point.Negative().BucketCounts()
	for i := 0; i < negativeBuckets.Len(); i++ {
		underflow += negativeBuckets.At(i)
	}

	// Next, pull in remaining buckets.
	counts := make([]int64, point.Positive().BucketCounts().Len()+2)
	bucketOptions := &distribution.Distribution_BucketOptions{}
	counts[0] = int64(underflow)
	positiveBuckets := point.Positive().BucketCounts()
	for i := 0; i < positiveBuckets.Len(); i++ {
		counts[i+1] = int64(positiveBuckets.At(i))
	}
	// Overflow bucket is always empty
	counts[len(counts)-1] = 0

	if point.Positive().BucketCounts().Len() == 0 {
		// We cannot send exponential distributions with no positive buckets,
		// instead we send a simple overflow/underflow histogram.
		bucketOptions.Options = &distribution.Distribution_BucketOptions_ExplicitBuckets{
			ExplicitBuckets: &distribution.Distribution_BucketOptions_Explicit{
				Bounds: []float64{0},
			},
		}
	} else {
		// Exponential histogram
		growth := math.Exp2(math.Exp2(-float64(point.Scale())))
		scale := math.Pow(growth, float64(point.Positive().Offset()))
		bucketOptions.Options = &distribution.Distribution_BucketOptions_ExponentialBuckets{
			ExponentialBuckets: &distribution.Distribution_BucketOptions_Exponential{
				GrowthFactor:     growth,
				Scale:            scale,
				NumFiniteBuckets: int32(len(counts) - 2),
			},
		}
	}

	mean := float64(0)
	if !math.IsNaN(point.Sum()) && point.Count() > 0 { // Avoid divide-by-zero
		mean = float64(point.Sum() / float64(point.Count()))
	}

	return &monitoringpb.TypedValue{
		Value: &monitoringpb.TypedValue_DistributionValue{
			DistributionValue: &distribution.Distribution{
				Count:         int64(point.Count()),
				Mean:          mean,
				BucketCounts:  counts,
				BucketOptions: bucketOptions,
				Exemplars:     m.exemplars(point.Exemplars(), projectID),
			},
		},
	}
}

func (m *metricMapper) histogramToTimeSeries(
	resource *monitoredrespb.MonitoredResource,
	extraLabels labels,
	metric pmetric.Metric,
	hist pmetric.Histogram,
	point pmetric.HistogramDataPoint,
	projectID string,
) []*monitoringpb.TimeSeries {
	if point.Flags().NoRecordedValue() || !point.HasSum() || point.ExplicitBounds().Len() == 0 {
		// Drop points without a value or without a sum
		m.obs.log.Debug("Metric has no value, sum, or explicit bounds. Dropping the metric.", zap.Any("metric", metric))
		return nil
	}
	t, err := m.metricNameToType(metric.Name(), metric)
	if err != nil {
		m.obs.log.Debug("Failed to get metric type (i.e. name) for histogram metric. Dropping the metric.", zap.Error(err), zap.Any("metric", metric))
		return nil
	}
	if hist.AggregationTemporality() == pmetric.AggregationTemporalityCumulative {
		// Normalize cumulative histogram points.
		metricIdentifier := datapointstorage.Identifier(resource, extraLabels, metric, point.Attributes())
		keep := m.normalizer.NormalizeHistogramDataPoint(point, metricIdentifier)
		if !keep {
			return nil
		}
	}

	// We treat deltas as cumulatives w/ resets.
	metricKind := metricpb.MetricDescriptor_CUMULATIVE
	startTime := timestamppb.New(point.StartTimestamp().AsTime())
	endTime := timestamppb.New(point.Timestamp().AsTime())
	value := m.histogramPoint(point, projectID)
	return []*monitoringpb.TimeSeries{{
		Resource:    resource,
		Unit:        metric.Unit(),
		Description: metric.Description(),
		MetricKind:  metricKind,
		ValueType:   metricpb.MetricDescriptor_DISTRIBUTION,
		Points: []*monitoringpb.Point{{
			Interval: &monitoringpb.TimeInterval{
				StartTime: startTime,
				EndTime:   endTime,
			},
			Value: value,
		}},
		Metric: &metricpb.Metric{
			Type: t,
			Labels: mergeLabels(
				attributesToLabels(point.Attributes()),
				extraLabels,
			),
		},
	}}
}

func (m *metricMapper) exponentialHistogramToTimeSeries(
	resource *monitoredrespb.MonitoredResource,
	extraLabels labels,
	metric pmetric.Metric,
	exponentialHist pmetric.ExponentialHistogram,
	point pmetric.ExponentialHistogramDataPoint,
	projectID string,
) []*monitoringpb.TimeSeries {
	if point.Flags().NoRecordedValue() {
		// Drop points without a value.
		return nil
	}
	t, err := m.metricNameToType(metric.Name(), metric)
	if err != nil {
		m.obs.log.Debug("Failed to get metric type (i.e. name) for exponential histogram metric. Dropping the metric.", zap.Error(err), zap.Any("metric", metric))
		return nil
	}
	if exponentialHist.AggregationTemporality() == pmetric.AggregationTemporalityCumulative {
		// Normalize the histogram point.
		metricIdentifier := datapointstorage.Identifier(resource, extraLabels, metric, point.Attributes())
		keep := m.normalizer.NormalizeExponentialHistogramDataPoint(point, metricIdentifier)
		if !keep {
			return nil
		}
	}
	// We treat deltas as cumulatives w/ resets.
	metricKind := metricpb.MetricDescriptor_CUMULATIVE
	startTime := timestamppb.New(point.StartTimestamp().AsTime())
	endTime := timestamppb.New(point.Timestamp().AsTime())
	value := m.exponentialHistogramPoint(point, projectID)
	return []*monitoringpb.TimeSeries{{
		Resource:    resource,
		Unit:        metric.Unit(),
		Description: metric.Description(),
		MetricKind:  metricKind,
		ValueType:   metricpb.MetricDescriptor_DISTRIBUTION,
		Points: []*monitoringpb.Point{{
			Interval: &monitoringpb.TimeInterval{
				StartTime: startTime,
				EndTime:   endTime,
			},
			Value: value,
		}},
		Metric: &metricpb.Metric{
			Type: t,
			Labels: mergeLabels(
				attributesToLabels(point.Attributes()),
				extraLabels,
			),
		},
	}}
}

func (m *metricMapper) sumPointToTimeSeries(
	resource *monitoredrespb.MonitoredResource,
	extraLabels labels,
	metric pmetric.Metric,
	sum pmetric.Sum,
	point pmetric.NumberDataPoint,
) []*monitoringpb.TimeSeries {
	metricKind := metricpb.MetricDescriptor_CUMULATIVE
	var startTime *timestamppb.Timestamp
	if point.Flags().NoRecordedValue() {
		// Drop points without a value.  This may be a staleness marker from
		// prometheus.
		return nil
	}
	t, err := m.metricNameToType(metric.Name(), metric)
	if err != nil {
		m.obs.log.Debug("Failed to get metric type (i.e. name) for sum metric. Dropping the metric.", zap.Error(err), zap.Any("metric", metric))
		return nil
	}
	if sum.IsMonotonic() {
		if sum.AggregationTemporality() == pmetric.AggregationTemporalityCumulative {
			metricIdentifier := datapointstorage.Identifier(resource, extraLabels, metric, point.Attributes())
			keep := m.normalizer.NormalizeNumberDataPoint(point, metricIdentifier)
			if !keep {
				return nil
			}
		}
		startTime = timestamppb.New(point.StartTimestamp().AsTime())
	} else {
		metricKind = metricpb.MetricDescriptor_GAUGE
		startTime = nil
	}
	value, valueType := m.numberDataPointToValue(point, metricKind, metric.Unit())

	return []*monitoringpb.TimeSeries{{
		Resource:    resource,
		Unit:        metric.Unit(),
		Description: metric.Description(),
		MetricKind:  metricKind,
		ValueType:   valueType,
		Points: []*monitoringpb.Point{{
			Interval: &monitoringpb.TimeInterval{
				StartTime: startTime,
				EndTime:   timestamppb.New(point.Timestamp().AsTime()),
			},
			Value: value,
		}},
		Metric: &metricpb.Metric{
			Type: t,
			Labels: mergeLabels(
				attributesToLabels(point.Attributes()),
				extraLabels,
			),
		},
	}}
}

func (m *metricMapper) gaugePointToTimeSeries(
	resource *monitoredrespb.MonitoredResource,
	extraLabels labels,
	metric pmetric.Metric,
	_ pmetric.Gauge,
	point pmetric.NumberDataPoint,
) []*monitoringpb.TimeSeries {
	if point.Flags().NoRecordedValue() {
		// Drop points without a value.
		return nil
	}
	t, err := m.metricNameToType(metric.Name(), metric)
	if err != nil {
		m.obs.log.Debug("Unable to get metric type (i.e. name) for gauge metric.", zap.Error(err), zap.Any("metric", metric))
		return nil
	}
	metricKind := metricpb.MetricDescriptor_GAUGE
	value, valueType := m.numberDataPointToValue(point, metricKind, metric.Unit())

	return []*monitoringpb.TimeSeries{{
		Resource:    resource,
		Unit:        metric.Unit(),
		Description: metric.Description(),
		MetricKind:  metricKind,
		ValueType:   valueType,
		Points: []*monitoringpb.Point{{
			Interval: &monitoringpb.TimeInterval{
				EndTime: timestamppb.New(point.Timestamp().AsTime()),
			},
			Value: value,
		}},
		Metric: &metricpb.Metric{
			Type: t,
			Labels: mergeLabels(
				attributesToLabels(point.Attributes()),
				extraLabels,
			),
		},
	}}
}

// Returns any configured prefix to add to unknown metric name.
func (m *metricMapper) getMetricNamePrefix(name string) string {
	for _, domain := range m.cfg.MetricConfig.KnownDomains {
		if strings.Contains(name, domain) {
			return ""
		}
	}
	return m.cfg.MetricConfig.Prefix
}

// metricNameToType maps OTLP metric name to GCM metric type (aka name).
func (m *metricMapper) metricNameToType(name string, metric pmetric.Metric) (string, error) {
	metricName, err := m.cfg.MetricConfig.GetMetricName(name, metric)
	if err != nil {
		return "", err
	}
	return path.Join(m.getMetricNamePrefix(metricName), metricName), nil
}

// defaultGetMetricName does not (further) customize the baseName.
func defaultGetMetricName(baseName string, _ pmetric.Metric) (string, error) {
	return baseName, nil
}

// this function converts the pdata metric to cloud monitoring.
func (m *metricMapper) numberDataPointToValue(
	point pmetric.NumberDataPoint,
	metricKind metricpb.MetricDescriptor_MetricKind,
	metricUnit string,
) (*monitoringpb.TypedValue, metricpb.MetricDescriptor_ValueType) {
	supportedTypedValue, supportedValueType := m.convertToBoolIfMetricKindSupported(point, metricKind, metricUnit)
	if supportedValueType != metricpb.MetricDescriptor_VALUE_TYPE_UNSPECIFIED {
		return supportedTypedValue, supportedValueType
	}
	if point.ValueType() == pmetric.NumberDataPointValueTypeInt {
		return &monitoringpb.TypedValue{Value: &monitoringpb.TypedValue_Int64Value{
				Int64Value: point.IntValue(),
			}},
			metricpb.MetricDescriptor_INT64
	}
	return &monitoringpb.TypedValue{Value: &monitoringpb.TypedValue_DoubleValue{
			DoubleValue: point.DoubleValue(),
		}},
		metricpb.MetricDescriptor_DOUBLE
}

// This function maps the values of the metric to certain types which are supported in Google Cloud Monitoring, but not in OTEL.
// Supported types includes BOOL. The conversion only happens for metric kind GAUGE and only if the conversion intent is indicated via the unit.
// The function returns the converted value and type if conditions are met, otherwise a nil value with value type MetricDescriptor_VALUE_TYPE_UNSPECIFIED is returned - indicating
// unsupported type or failure to meet constraints for conversion.
func (me *metricMapper) convertToBoolIfMetricKindSupported(
	point pmetric.NumberDataPoint,
	metricKind metricpb.MetricDescriptor_MetricKind,
	metricUnit string,
) (*monitoringpb.TypedValue, metricpb.MetricDescriptor_ValueType) {
	if metricUnit == specialIntToBoolUnit {
		if metricKind == metricpb.MetricDescriptor_GAUGE && point.ValueType() == pmetric.NumberDataPointValueTypeInt {
			return &monitoringpb.TypedValue{Value: &monitoringpb.TypedValue_BoolValue{
					BoolValue: point.IntValue() != 0,
				}},
				metricpb.MetricDescriptor_BOOL
		}
		me.obs.log.Warn("Failed to interpret metric as BOOL. Attempted conversion on BOOL metrics are only supported on integer valued gauges", zap.Any("metric_kind", metricKind), zap.Any("value_type", point.ValueType()))
	}
	return nil, metricpb.MetricDescriptor_VALUE_TYPE_UNSPECIFIED
}

func attributesToLabels(attrs pcommon.Map) labels {
	ls := make(labels, attrs.Len())
	attrs.Range(func(k string, v pcommon.Value) bool {
		ls[sanitizeKey(k)] = sanitizeUTF8(v.AsString())
		return true
	})
	return ls
}

func sanitizeUTF8(s string) string {
	return strings.ToValidUTF8(s, "�")
}

// Replaces non-alphanumeric characters to underscores. Note, this does not truncate label keys
// longer than 100 characters or prepend "key" when the first character is "_" like OpenCensus
// did.
func sanitizeKey(s string) string {
	if len(s) == 0 {
		return s
	}
	s = strings.Map(sanitizeRune, s)
	if unicode.IsDigit(rune(s[0])) {
		s = "key_" + s
	}
	return s
}

// converts anything that is not a letter or digit to an underscore.
func sanitizeRune(r rune) rune {
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return r
	}
	// Everything else turns into an underscore
	return '_'
}

func mergeLabels(mergeInto labels, others ...labels) labels {
	if mergeInto == nil {
		mergeInto = labels{}
	}
	for _, ls := range others {
		for k, v := range ls {
			mergeInto[k] = v
		}
	}

	return mergeInto
}

// Takes a GCM metric type, like (workload.googleapis.com/MyCoolMetric) and returns the display name.
func (m *metricMapper) metricTypeToDisplayName(mURL string) string {
	// TODO - user configuration around display name?
	// Default: strip domain, keep path after domain.
	u, err := url.Parse(fmt.Sprintf("metrics://%s", mURL))
	if err != nil || u.Path == "" {
		return mURL
	}
	return strings.TrimLeft(u.Path, "/")
}

// Returns label descriptors for a metric.
func (m *metricMapper) labelDescriptors(
	pm pmetric.Metric,
	extraLabels labels,
) []*label.LabelDescriptor {
	// TODO - allow customization of label descriptions.
	result := []*label.LabelDescriptor{}
	for key := range extraLabels {
		result = append(result, &label.LabelDescriptor{
			Key: sanitizeKey(key),
		})
	}

	seenKeys := map[string]struct{}{}
	addAttributes := func(attr pcommon.Map) {
		attr.Range(func(key string, _ pcommon.Value) bool {
			// Skip keys that have already been set
			if _, ok := seenKeys[sanitizeKey(key)]; ok {
				return true
			}
			result = append(result, &label.LabelDescriptor{
				Key: sanitizeKey(key),
			})
			seenKeys[sanitizeKey(key)] = struct{}{}
			return true
		})
	}
	switch pm.Type() {
	case pmetric.MetricTypeGauge:
		points := pm.Gauge().DataPoints()
		for i := 0; i < points.Len(); i++ {
			addAttributes(points.At(i).Attributes())
		}
	case pmetric.MetricTypeSum:
		points := pm.Sum().DataPoints()
		for i := 0; i < points.Len(); i++ {
			addAttributes(points.At(i).Attributes())
		}
	case pmetric.MetricTypeSummary:
		points := pm.Summary().DataPoints()
		for i := 0; i < points.Len(); i++ {
			addAttributes(points.At(i).Attributes())
		}
	case pmetric.MetricTypeHistogram:
		points := pm.Histogram().DataPoints()
		for i := 0; i < points.Len(); i++ {
			addAttributes(points.At(i).Attributes())
		}
	case pmetric.MetricTypeExponentialHistogram:
		points := pm.ExponentialHistogram().DataPoints()
		for i := 0; i < points.Len(); i++ {
			addAttributes(points.At(i).Attributes())
		}
	}
	return result
}

// Returns (sum, count, quantile) metric types (i.e. names) for a summary metric.
func (m *metricMapper) summaryMetricTypes(pm pmetric.Metric) (string, string, string, error) {
	sumType, err := m.metricNameToType(pm.Name()+SummarySumSuffix, pm)
	if err != nil {
		return "", "", "", err
	}
	countType, err := m.metricNameToType(pm.Name()+SummaryCountPrefix, pm)
	if err != nil {
		return "", "", "", err
	}
	quantileType, err := m.metricNameToType(pm.Name(), pm)
	if err != nil {
		return "", "", "", err
	}
	return sumType, countType, quantileType, nil
}

func (m *metricMapper) summaryMetricDescriptors(
	pm pmetric.Metric,
	extraLabels labels,
) []*metricpb.MetricDescriptor {
	sumType, countType, quantileType, err := m.summaryMetricTypes(pm)
	if err != nil {
		m.obs.log.Debug("Failed to get metric types (i.e. names) for summary metric. Dropping the metric.", zap.Error(err), zap.Any("metric", pm))
		return nil
	}
	labels := m.labelDescriptors(pm, extraLabels)
	return []*metricpb.MetricDescriptor{
		{
			Type:        sumType,
			Labels:      labels,
			MetricKind:  metricpb.MetricDescriptor_CUMULATIVE,
			ValueType:   metricpb.MetricDescriptor_DOUBLE,
			Unit:        pm.Unit(),
			Description: pm.Description(),
			DisplayName: pm.Name() + SummarySumSuffix,
		},
		{
			Type:        countType,
			Labels:      labels,
			MetricKind:  metricpb.MetricDescriptor_CUMULATIVE,
			ValueType:   metricpb.MetricDescriptor_DOUBLE,
			Unit:        pm.Unit(),
			Description: pm.Description(),
			DisplayName: pm.Name() + SummaryCountPrefix,
		},
		{
			Type: quantileType,
			Labels: append(
				labels,
				&label.LabelDescriptor{
					Key:         "quantile",
					Description: "the value at a given quantile of a distribution",
				}),
			MetricKind:  metricpb.MetricDescriptor_GAUGE,
			ValueType:   metricpb.MetricDescriptor_DOUBLE,
			Unit:        pm.Unit(),
			Description: pm.Description(),
			DisplayName: pm.Name(),
		},
	}
}

// Extract the metric descriptor from a metric data point.
func (m *metricMapper) metricDescriptor(
	pm pmetric.Metric,
	extraLabels labels,
) []*metricpb.MetricDescriptor {
	if pm.Type() == pmetric.MetricTypeSummary {
		return m.summaryMetricDescriptors(pm, extraLabels)
	}
	kind, typ := m.mapMetricPointKind(pm)
	if kind == metricpb.MetricDescriptor_METRIC_KIND_UNSPECIFIED {
		m.obs.log.Debug("Failed to get metric kind (i.e. aggregation) for metric descriptor. Dropping the metric descriptor.", zap.Any("metric", pm))
		return nil
	}
	if typ == metricpb.MetricDescriptor_VALUE_TYPE_UNSPECIFIED {
		m.obs.log.Debug("Failed to get metric type (int / double) for metric descriptor. Dropping the metric descriptor.", zap.Any("metric", pm))
		return nil
	}
	metricType, err := m.metricNameToType(pm.Name(), pm)
	if err != nil {
		m.obs.log.Debug("Failed to get metric type (i.e. name) for metric descriptor. Dropping the metric descriptor.", zap.Error(err), zap.Any("metric", pm))
		return nil
	}
	labels := m.labelDescriptors(pm, extraLabels)
	// Return nil for unsupported types.
	if kind == metricpb.MetricDescriptor_METRIC_KIND_UNSPECIFIED {
		return nil
	}
	return []*metricpb.MetricDescriptor{
		{
			Name:        pm.Name(),
			DisplayName: m.metricTypeToDisplayName(metricType),
			Type:        metricType,
			MetricKind:  kind,
			ValueType:   typ,
			Unit:        pm.Unit(),
			Description: pm.Description(),
			Labels:      labels,
		},
	}
}

func metricPointValueType(pt pmetric.NumberDataPointValueType) metricpb.MetricDescriptor_ValueType {
	switch pt {
	case pmetric.NumberDataPointValueTypeInt:
		return metricpb.MetricDescriptor_INT64
	case pmetric.NumberDataPointValueTypeDouble:
		return metricpb.MetricDescriptor_DOUBLE
	default:
		return metricpb.MetricDescriptor_VALUE_TYPE_UNSPECIFIED
	}
}

func isEmptyReq(req *monitoringpb.CreateTimeSeriesRequest) bool {
	return (req == nil || (req.Name == "" && req.TimeSeries == nil))
}

func (me *metricMapper) mapMetricPointKind(m pmetric.Metric) (metricpb.MetricDescriptor_MetricKind, metricpb.MetricDescriptor_ValueType) {
	var kind metricpb.MetricDescriptor_MetricKind
	var typ metricpb.MetricDescriptor_ValueType
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		kind = metricpb.MetricDescriptor_GAUGE
		if m.Gauge().DataPoints().Len() > 0 {
			_, supportedType := me.convertToBoolIfMetricKindSupported(m.Gauge().DataPoints().At(0), kind, m.Unit())
			if supportedType != metricpb.MetricDescriptor_VALUE_TYPE_UNSPECIFIED {
				typ = supportedType
			} else {
				typ = metricPointValueType(m.Gauge().DataPoints().At(0).ValueType())
			}
		}
	case pmetric.MetricTypeSum:
		if !m.Sum().IsMonotonic() {
			kind = metricpb.MetricDescriptor_GAUGE
		} else {
			kind = metricpb.MetricDescriptor_CUMULATIVE
		}
		if m.Sum().DataPoints().Len() > 0 {
			typ = metricPointValueType(m.Sum().DataPoints().At(0).ValueType())
		}
	case pmetric.MetricTypeSummary:
		kind = metricpb.MetricDescriptor_GAUGE
	case pmetric.MetricTypeHistogram:
		typ = metricpb.MetricDescriptor_DISTRIBUTION
		kind = metricpb.MetricDescriptor_CUMULATIVE
	case pmetric.MetricTypeExponentialHistogram:
		typ = metricpb.MetricDescriptor_DISTRIBUTION
		kind = metricpb.MetricDescriptor_CUMULATIVE
	default:
		kind = metricpb.MetricDescriptor_METRIC_KIND_UNSPECIFIED
		typ = metricpb.MetricDescriptor_VALUE_TYPE_UNSPECIFIED
	}
	return kind, typ
}
