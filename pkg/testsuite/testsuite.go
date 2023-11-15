package testsuite

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/prometheus/client_golang/api"
	apiv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// Suite holds a dbtest.Suite and a lazily started prometheus server.
// Each Suite holds its own Prometheus server. Suites can be run in parallel.
type Suite struct {
	suite.Suite
	// promMutex guards all prom* variables below.
	promMutex        sync.Mutex
	promAddr         string
	promCtx          context.Context
	promWaitShutdown func() error
	promCancelCtx    context.CancelFunc
}

// PrometheusURL starts a prometheus server and returns the api url to it.
// The started prometheus is shared in a testsuite.
func (ts *Suite) PrometheusURL() string {
	ts.promMutex.Lock()
	defer ts.promMutex.Unlock()
	if ts.promAddr != "" {
		return ts.promAddr
	}

	port, err := getFreePort()
	require.NoError(ts.T(), err)
	url := fmt.Sprintf("http://127.0.0.1:%d", port)

	ts.promCtx, ts.promCancelCtx = context.WithCancel(context.Background())
	ts.Require().FileExists(PromBin, "Prometheus binary is required to run tests. Download with `make ensure-prometheus`")
	wait, err := StartPrometheus(ts.promCtx, port)
	require.NoError(ts.T(), err)
	ts.promWaitShutdown = wait

	require.NoError(ts.T(), waitForPrometheusReady(ts.promCtx, url, 5*time.Second))

	ts.promAddr = url
	return url
}

// PrometheusAPIClient starts a prometheus server and returns a client to it.
// The started prometheus is shared in a testsuite.
func (ts *Suite) PrometheusAPIClient() apiv1.API {
	client, err := api.NewClient(api.Config{Address: ts.PrometheusURL()})
	require.NoError(ts.T(), err)
	return apiv1.NewAPI(client)
}

// TearDownSuite stops prometheus and drops the temporary database.
func (ts *Suite) TearDownSuite() {
	ts.promMutex.Lock()
	defer ts.promMutex.Unlock()

	if ts.promCancelCtx != nil {
		defer ts.promWaitShutdown()
		ts.promCancelCtx()
	}
}

func waitForPrometheusReady(ctx context.Context, url string, timeout time.Duration) error {
	client, err := api.NewClient(api.Config{Address: url})
	if err != nil {
		return fmt.Errorf("error creating prometheus client: %w", err)
	}
	clientv1 := apiv1.NewAPI(client)
	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, timeout)
	defer timeoutCancel()
	for {
		_, err := clientv1.Runtimeinfo(timeoutCtx)
		if err == nil {
			break
		} else if timeoutCtx.Err() != nil {
			return fmt.Errorf("failed waiting for prometheus to become ready, last error: %w", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

func getFreePort() (port int, err error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
