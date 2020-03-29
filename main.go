package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics/prometheus"
	"github.com/go-kit/kit/sd"
	"github.com/gorilla/handlers"
	"github.com/hashicorp/consul/api"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gokit_outbound/base"
	"log/syslog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

var (
	serviceName   = flag.String("service.name", "user", "Name of micro service")
	serviceGroup  = flag.String("service.group", "user", "service group of metrics")
	basePath      = flag.String("service.base.path", "user", "base path")
	version       = flag.String("service.version", "v1", "version of micro service (default:v1)")
	dataType      = flag.String("service.datatype", "test", "default is TEST")
	httpPort      = flag.Int("http.port", 8080, "this port ar which http requests are accepted")
	httpAddr      = flag.String("http.addr", "localhost", "this port ar which htto requests are accepted")
	metricsPort   = flag.Int("metrics.port", 8082, "HTTP metrics listening port")
	consulAddr    = flag.String("consul.addr", "localhost:8500", "consul add, default:8500")
	serverTimeout = flag.Int64("server.timeout", 1500, "server time out in milliseconds")
	sysLogAddress = flag.String("syslog.address", "localhost:514", "default location for the sysLogger")
)

func main() {
	flag.Parse()

	logger, service, err := initApp()
	if err != nil {
		fmt.Println("failed to initialize the application :: %s", err.Error())
	}

	errs := make(chan error)

	httpServer := initHTTPServer(logger, service)
	{
		go func() {
			logger.Log("transport", "HTTP", "addr", *httpPort)
			errs <- httpServer.ListenAndServe()
		}()
	}

	metricsServer := initMetricsServer()
	{
		go func() {
			logger.Log("transport", "HTTP", "addr", *metricsPort)
			errs <- metricsServer.ListenAndServe()
		}()
	}

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errs <- fmt.Errorf("%s", <-c)
	}()

	mainErr := <-errs
	errMetricsServer := metricsServer.Shutdown(context.Background())
	errHTTPServer := httpServer.Shutdown(context.Background())
	logger.Log("exit : ", mainErr, " http_Error : ", errHTTPServer, " metrics_Error : ", errMetricsServer)
}

func initHTTPServer(logger log.Logger, s base.Service) http.Server {
	labelNames := []string{"path", "code"}
	constLables := map[string]string{"serviceName": *basePath, "serviceGroup": *serviceGroup, "version": *version, "dataType": *dataType}

	transport := base.NewTransportServerFinalizerInstrument(
		labelNames,
		prometheus.NewHistogramFrom(
			stdprometheus.HistogramOpts{
				Name:        "transport_request_latency",
				Help:        "Duration of request in seconds",
				ConstLabels: constLables,
				Buckets:     []float64{0.01, 0.025, 0.05, 0.1, 0.3, 0.6, 1},
			},
			labelNames,
		),
	)
	h := base.NewHTTPHandler(s, *version, *basePath, transport)
	h = http.TimeoutHandler(h, time.Duration(*serverTimeout)*time.Millisecond, "")
	return http.Server{
		Addr:    ":" + strconv.Itoa(*httpPort),
		Handler: handlers.RecoveryHandler(handlers.RecoveryLogger(base.NewPanicLogger(logger)))(h),
	}
}
func initApp() (log.Logger, base.Service, error) {
	initVersion()

	logger, err := initLogger()
	if err != nil {
		return logger, nil, err
	}

	_, registrar, err := initRegister(logger)
	if err != nil {
		return logger, nil, err
	}
	registrar.Register()
	s := initService(logger)
	return logger, s, nil
}

func initVersion() {
	if *version == "" {
		if versionFlag := flag.Lookup("version"); versionFlag != nil {
			flag.Set("version", versionFlag.DefValue)
		}

	}
}

func initLogger() (log.Logger, error) {
	var logger log.Logger

	sysLogger, err := syslog.Dial("udp", *sysLogAddress, syslog.LOG_EMERG|syslog.LOG_LOCAL6, *serviceName)
	if err != nil {
		return logger, err
	}
	{
		logger = log.NewJSONLogger(sysLogger)
		logger = log.With(logger, "ip", *httpAddr)
		logger = log.With(logger, "serviceName", *serviceName)
		logger = log.With(logger, "version", *version)
		logger = log.With(logger, "dataType", *dataType)
		logger = log.With(logger, "ts", log.DefaultTimestampUTC)
		logger = log.With(logger, "caller", log.DefaultCaller)
	}
	defer sysLogger.Close()
	return logger, err
}

func initRegister(logger log.Logger) (*api.Client, sd.Registrar, error) {
	register := base.ServiceRegistration{
		ServiceName:   *serviceName,
		ConsulAddress: *consulAddr,
		HTTPAddress:   *httpAddr,
		HTTPPort:      *httpPort,
	}
	return base.Register(register, logger)
}

func initService(logger log.Logger) base.Service {
	labelNames := []string{"method"}
	constLabels := map[string]string{"serviceName": *basePath, "serviceGroup": *serviceGroup, "version": *version, "dataType": *dataType}

	var s base.Service

	s = base.NewService()
	s = base.NewLoggingMiddleware(logger)(s)
	s = base.NewInstrumentingService(
		labelNames,
		prometheus.NewCounterFrom(
			stdprometheus.CounterOpts{
				Name:        "request_count",
				Help:        "Number of request received",
				ConstLabels: constLabels,
			},
			labelNames,
		),
		prometheus.NewCounterFrom(
			stdprometheus.CounterOpts{
				Name:        "err_count",
				Help:        "Number of errors",
				ConstLabels: constLabels,
			},
			labelNames,
		),
		prometheus.NewSummaryFrom(
			stdprometheus.SummaryOpts{
				Name:        "request_latency_seconds",
				Help:        "Total duration of requests in request_latency_seconds",
				ConstLabels: constLabels,
			},
			labelNames,
		),
		prometheus.NewHistogramFrom(
			stdprometheus.HistogramOpts{
				Name:        "request_latency",
				Help:        "Duration of request in seconds",
				ConstLabels: constLabels,
			},
			labelNames,
		),
	)(s)

	return s
}

func initMetricsServer() http.Server {
	return http.Server{
		Addr:    ":" + strconv.Itoa(*metricsPort),
		Handler: promhttp.Handler(),
	}
}
