package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/mux"
	"github.com/kenshaw/envcfg"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	grpctrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/google.golang.org/grpc"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	"gopkg.in/DataDog/dd-trace-go.v1/profiler"

	"github.com/dropezy/internal/logging"
	"github.com/dropezy/storefront-backend/http/callback/midtrans"
	"github.com/dropezy/storefront-backend/http/callback/mileapp"
	"github.com/dropezy/storefront-backend/http/callback/shoptree"

	// protobuf

	inpb "github.com/dropezy/proto/v1/inventory"
	opb "github.com/dropezy/proto/v1/order"
	tpb "github.com/dropezy/proto/v1/task"
)

const service = "http-server"

var (
	version     = "development"
	environment = "development"

	config *envcfg.Envcfg
	logger zerolog.Logger
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	var err error
	config, err = envcfg.New()
	if err != nil {
		log.Fatal(err)
	}

	environment = config.GetString("runtime.environment")

	logLevel := zerolog.InfoLevel
	levelStr := config.GetString("log.level")
	if levelStr == "fromenv" {
		switch environment {
		case "staging", "development":
			logLevel = zerolog.DebugLevel
		}
	} else {
		var err error
		logLevel, err = zerolog.ParseLevel(levelStr)
		if err != nil {
			log.Fatal(err)
		}
	}

	logger = logging.NewLogger().
		Level(logLevel).With().
		Str("service-name", service).
		Str("version", version).
		Logger()
}

func main() {
	logger := logger.With().Str("func", "main").Logger()

	if environment == "production" {
		// TODO(vishen): move datadog tracing and profile stuff to an importable package
		// Start datadog APM
		tracer.Start(
			tracer.WithEnv(environment),
			tracer.WithService(service),
			tracer.WithServiceVersion(version),
			tracer.WithAgentAddr(config.GetString("datadog.agentAddr")),
		)
		defer tracer.Stop()

		err := profiler.Start(
			profiler.WithService(service),
			profiler.WithEnv(environment),
			profiler.WithVersion(version),
			profiler.WithAgentAddr(config.GetString("datadog.agentAddr")),
			profiler.WithProfileTypes(
				profiler.CPUProfile,
				profiler.HeapProfile,
				profiler.GoroutineProfile,
			),
		)
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to set up datadog profiler")
		}
		defer profiler.Stop()
	}

	// GRPC client
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(
			grpctrace.UnaryClientInterceptor(grpctrace.WithServiceName(service)),
			storefrontAuthInterceptor,
		),
	}

	// The server address in the format of host:port
	conn, err := grpc.Dial(config.GetString("grpc.addr"), opts...)
	if err != nil {
		logger.Fatal().Msgf("fail to dial: %v", err)
	}
	defer conn.Close()

	var (
		orderClient     = opb.NewOrderServiceClient(conn)
		taskClient      = tpb.NewTaskServiceClient(conn)
		inventoryClient = inpb.NewInventoryServiceClient(conn)
	)

	addr := net.JoinHostPort("", config.GetString("server.port"))
	srv := &http.Server{
		Addr:         addr,
		Handler:      registerHandler(orderClient, taskClient, inventoryClient),
		ReadTimeout:  config.GetDuration("server.readTimeout"),
		IdleTimeout:  config.GetDuration("server.idleTimeout"),
		WriteTimeout: config.GetDuration("server.writeTimeout"),
	}

	idleConnsClosed := make(chan struct{})
	// watch for os.Interrupt signal and gracefully shutdown
	// the server.
	go func() {
		const shutdownTimeout = 10 * time.Second

		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		<-sigint

		ctx, cancel := context.WithTimeout(
			context.Background(),
			shutdownTimeout,
		)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			logger.Fatal().Err(err).Msg("HTTP server shutdown")
		}
		logger.Info().Msg("HTTP server shutdown")
		close(idleConnsClosed)
	}()

	logger.Info().Msgf("starting %s on port:%s", service, addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		logger.Fatal().Err(err).Msg("HTTP server ListenAndServe")
	}
	<-idleConnsClosed

	logger.Info().Msg("server exited gracefully")
}

func registerHandler(
	orderClient opb.OrderServiceClient,
	taskClient tpb.TaskServiceClient,
	inventoryClient inpb.InventoryServiceClient,
) http.Handler {
	router := mux.NewRouter()

	// Add default handler as fallback
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write(
			[]byte(fmt.Sprintf("%s at version, %s", service, version)),
		)
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to write to default fallback")
		}
	})

	// MileApp handlers
	mileappHandlers := mileapp.NewMileappHandlers(
		config.GetString("mileapp.authKey"), taskClient,
	)
	mileappRouter := router.PathPrefix("/mileapp").Subrouter()
	mileappRouter.HandleFunc("/status/{task-type}", mileappHandlers.HandleStatusUpdate)

	// Shoptree handlers
	shoptreeHandlers, err := shoptree.NewHandler(
		config.GetString("shoptree.authKey"), inventoryClient,
	)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to initialize shoptree handler")
	}
	shoptreeRouter := router.PathPrefix("/shoptree").Subrouter()
	shoptreeRouter.HandleFunc("/stock-update", shoptreeHandlers.HandleStockUpdate)
	shoptreeRouter.HandleFunc("/product-status-update", shoptreeHandlers.HandleProductStatusUpdate)

	// Midtrans handlers
	midtransHandlers, err := midtrans.NewHandler(config.GetString("midtrans.serverKey"),
		config.GetString("midtrans.chargeURL"),
		config.GetString("midtrans.getStatusURL"),
		orderClient, taskClient)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to initialize midtrans handler")
	}
	midtransRouter := router.PathPrefix("/midtrans").Subrouter()
	midtransRouter.HandleFunc("/transaction-update", midtransHandlers.HandleTransactionUpdate)

	return router
}

func storefrontAuthInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	authCtx := metadata.AppendToOutgoingContext(ctx, "x-api-Key", config.GetString("storefront-api.authKey"))
	return invoker(authCtx, method, req, reply, cc, opts...)
}
