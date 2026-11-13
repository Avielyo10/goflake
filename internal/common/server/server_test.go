package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Avielyo10/goflake/config"
	pb "github.com/Avielyo10/goflake/internal/common/genproto/api/protobuf"
)

func testConfig() *config.Config {
	var cfg config.Config
	cfg.Env = config.ProductionEnvType
	cfg.DatacenterID = 3
	cfg.MachineID = 17
	cfg.Flake.Epoch = 1659034655453
	cfg.Flake.TickMs = 1
	cfg.Flake.BitsLen.DatacenterID = 5
	cfg.Flake.BitsLen.MachineID = 5
	cfg.Flake.BitsLen.Sequence = 12
	cfg.Flake.BitsLen.Time = 41
	return &cfg
}

func testLogger() *logrus.Logger {
	log := logrus.New()
	log.SetOutput(io.Discard)
	return log
}

// get performs a GET request against the rest server and decodes the json response
func get(t *testing.T, s *RESTServer, path string, header http.Header) (int, map[string]any) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router, err := s.router()
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range header {
		req.Header[k] = v
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	body := map[string]any{}
	dec := json.NewDecoder(rec.Body)
	dec.UseNumber()
	if err := dec.Decode(&body); err != nil {
		t.Fatalf("GET %s: invalid json response: %v", path, err)
	}
	return rec.Code, body
}

func TestRESTUUIDAndDecompose(t *testing.T) {
	cfg := testConfig()
	s := NewRESTServer(cfg, testLogger())

	code, body := get(t, s, "/v1/uuid", nil)
	if code != http.StatusOK {
		t.Fatalf("GET /v1/uuid = %d %v", code, body)
	}
	uuid := body["uuid"].(json.Number).String()

	before := time.Now()
	code, body = get(t, s, "/v1/decompose/"+uuid, nil)
	if code != http.StatusOK {
		t.Fatalf("GET /v1/decompose = %d %v", code, body)
	}
	want := map[string]string{"uuid": uuid, "msb": "0", "datacenter_id": "3", "machine_id": "17", "sequence": "0"}
	for k, v := range want {
		if got := body[k].(json.Number).String(); got != v {
			t.Errorf("%s = %s, want %s", k, got, v)
		}
	}
	timestamp, err := time.Parse(time.RFC3339, body["timestamp"].(string))
	if err != nil {
		t.Fatalf("timestamp is not RFC3339: %v", err)
	}
	if d := before.Sub(timestamp); d < -time.Second || d > time.Minute {
		t.Errorf("timestamp = %v, want it around %v", timestamp, before)
	}
}

func TestRESTDecomposeInvalidUUID(t *testing.T) {
	s := NewRESTServer(testConfig(), testLogger())
	for _, uuid := range []string{"abc", "-1", "18446744073709551616"} {
		code, body := get(t, s, "/v1/decompose/"+uuid, nil)
		if code != http.StatusBadRequest {
			t.Errorf("GET /v1/decompose/%s = %d, want %d", uuid, code, http.StatusBadRequest)
		}
		// the error used to report the parsed value (0) instead of the input
		if want := "invalid uuid: " + uuid; body["error"] != want {
			t.Errorf("error = %q, want %q", body["error"], want)
		}
	}
}

func TestRESTUUIDUnavailable(t *testing.T) {
	cfg := testConfig()
	cfg.Flake.Epoch = uint64(time.Now().Add(time.Hour).UnixMilli())
	s := NewRESTServer(cfg, testLogger())

	if code, body := get(t, s, "/v1/uuid", nil); code != http.StatusServiceUnavailable || body["error"] == nil {
		t.Fatalf("GET /v1/uuid = %d %v, want %d with an error", code, body, http.StatusServiceUnavailable)
	}
}

func TestRESTIgnoresForwardedFor(t *testing.T) {
	s := NewRESTServer(testConfig(), testLogger())
	gin.SetMode(gin.TestMode)
	router, err := s.router()
	if err != nil {
		t.Fatal(err)
	}
	var clientIP string
	router.GET("/ip", func(c *gin.Context) { clientIP = c.ClientIP() })

	req := httptest.NewRequest(http.MethodGet, "/ip", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.99")
	router.ServeHTTP(httptest.NewRecorder(), req)
	if clientIP != "192.0.2.10" {
		t.Fatalf("ClientIP() = %q, want the peer address 192.0.2.10", clientIP)
	}
}

func TestGRPCUUIDAndDecompose(t *testing.T) {
	s := NewGRPCServer(testConfig(), testLogger())
	ctx := context.Background()

	before := time.Now()
	uuid, err := s.GetUUID(ctx, &pb.GetUUIDRequest{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.Decompose(ctx, &pb.DecomposeRequest{Uuid: uuid.Uuid})
	if err != nil {
		t.Fatal(err)
	}
	if res.DatacenterId != "3" || res.MachineId != "17" || res.Sequence != "0" || res.Msb {
		t.Errorf("Decompose() = %v", res)
	}
	// the proto documents the timestamp as RFC3339
	timestamp, err := time.Parse(time.RFC3339, res.Timestamp)
	if err != nil {
		t.Fatalf("timestamp is not RFC3339: %v", err)
	}
	if d := before.Sub(timestamp); d < -time.Second || d > time.Minute {
		t.Errorf("timestamp = %v, want it around %v", timestamp, before)
	}

	if res, err = s.Decompose(ctx, &pb.DecomposeRequest{Uuid: 1<<64 - 1}); err != nil || !res.Msb {
		t.Errorf("Decompose(max) = %v, %v, want msb set", res, err)
	}
}

func TestGRPCUUIDUnavailable(t *testing.T) {
	cfg := testConfig()
	cfg.Flake.Epoch = uint64(time.Now().Add(time.Hour).UnixMilli())
	s := NewGRPCServer(cfg, testLogger())

	if _, err := s.GetUUID(context.Background(), &pb.GetUUIDRequest{}); status.Code(err) != codes.Unavailable {
		t.Fatalf("GetUUID() error = %v, want code %v", err, codes.Unavailable)
	}
}
