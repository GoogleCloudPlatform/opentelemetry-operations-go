package  exporter_test 


import (
	"context"
	"flag"
	"testing"
	"log"
	"net"
	"sync"
	"os"
	"time"



	"google.golang.org/grpc"
	"google.golang.org/api/option"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/api/global"
	
	tracepb "google.golang.org/genproto/googleapis/devtools/cloudtrace/v2"
	emptypb "github.com/golang/protobuf/ptypes/empty"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	//texporter "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
	exporter "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter"
)


type mockTraceServer struct {
	tracepb.TraceServiceServer
	mu            sync.Mutex
	spansUploaded []*tracepb.Span
	delay         time.Duration
}

func (s *mockTraceServer) BatchWriteSpans(ctx context.Context, req *tracepb.BatchWriteSpansRequest) (*emptypb.Empty, error) {
	var err error
	s.mu.Lock()
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case <-time.After(s.delay):
		s.spansUploaded = append(s.spansUploaded, req.Spans...)
	}
	s.mu.Unlock()
	return &emptypb.Empty{}, err
}

func (s *mockTraceServer) len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.spansUploaded)
}

// clientOpt is the option tests should use to connect to the test server.
// It is initialized by TestMain.
var clientOpt []option.ClientOption

var (
	mockTrace mockTraceServer
)


func TestMain(m *testing.M) {
	flag.Parse()

	serv := grpc.NewServer()
	tracepb.RegisterTraceServiceServer(serv, &mockTrace)

	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		_ = serv.Serve(lis)
	}()

	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}
	clientOpt = []option.ClientOption{option.WithGRPCConn(conn)}
	os.Exit(m.Run())
}


func TestExporter_ExportSpan(t *testing.T) {	
	mockTrace.spansUploaded = nil
	mockTrace.delay = 0

	// Create Google Cloud Trace Exporter
	exp, err := exporter.NewExporter(
		exporter.Options{
			ProjectID: "PROJECT_ID_NOT_REAL",		
			TraceClientOptions: clientOpt,
		},
	)
	assert.NoError(t, err)

	tp, err := sdktrace.NewProvider(
		sdktrace.WithConfig(sdktrace.Config{DefaultSampler: sdktrace.AlwaysSample()}),
		sdktrace.WithBatcher(exp.GetTraceExporter(), // add following two options to ensure flush
			sdktrace.WithBatchTimeout(1),
			sdktrace.WithMaxExportBatchSize(1),
		))
	assert.NoError(t, err)

	global.SetTraceProvider(tp)
	_, span := global.TraceProvider().Tracer("test-tracer").Start(context.Background(), "test-span")
	span.End()
	assert.True(t, span.SpanContext().IsValid())

	// wait exporter to flush
	time.Sleep(20 * time.Millisecond)
	assert.EqualValues(t, 1, mockTrace.len())
}

func TestExporter_injectLabelsIntoSpan(t *testing.T) {
	
}
