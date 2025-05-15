package main

import (
	"bank-ledger/internal/biz"
	"bank-ledger/internal/data"
	"bank-ledger/internal/kafka"
	"bank-ledger/internal/server"
	"bank-ledger/internal/service"
	"flag"
	"os"

	"bank-ledger/internal/conf"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"

	_ "go.uber.org/automaxprocs"
)

// go build -ldflags "-X main.Version=x.y.z"
var (
	// Name is the name of the compiled software.
	Name string
	// Version is the version of the compiled software.
	Version string
	// flagconf is the config flag.
	flagconf string

	id, _ = os.Hostname()
)

func init() {
	flag.StringVar(&flagconf, "conf", "./configs", "config path, eg: -conf config.yaml")
}

func newApp(logger log.Logger, gs *grpc.Server, hs *http.Server) *kratos.App {
	return kratos.New(
		kratos.ID(id),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{}),
		kratos.Logger(logger),
		kratos.Server(
			gs,
			hs,
		),
	)
}

func main() {
	flag.Parse()
	logger := log.With(log.NewStdLogger(os.Stdout),
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"service.id", id,
		"service.name", Name,
		"service.version", Version,
		"trace.id", tracing.TraceID(),
		"span.id", tracing.SpanID(),
	)
	c := config.New(
		config.WithSource(
			file.NewSource(flagconf),
		),
	)
	defer c.Close()

	if err := c.Load(); err != nil {
		panic(err)
	}

	var bc conf.Bootstrap
	if err := c.Scan(&bc); err != nil {
		panic(err)
	}

	app, cleanup, err := wireApp(bc.Server, bc.Data, logger)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	// start and wait for stop signal
	if err := app.Run(); err != nil {
		panic(err)
	}
}

func wireApp(confServer *conf.Server, confData *conf.Data, logger log.Logger) (*kratos.App, func(), error) {
	dataData, cleanup, err := data.NewData(confData, logger)
	if err != nil {
		return nil, nil, err
	}
	accountRepository := data.NewAccountRepo(dataData, logger)
	accountHandler := biz.NewAccountHandler(accountRepository, logger)
	accountService := service.NewAccountService(accountHandler)
	producer, err := kafka.NewProducer(confData, logger)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	transactionRepository := data.NewTransactionRepo(dataData, logger)
	database, cleanup2, err := data.NewMongoDBConnection(confData, logger)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	transactionLogsRepository := data.NewTransactionLogsRepo(dataData, logger, database)
	transactionHandler := biz.NewTransactionHandler(producer, logger, accountRepository, transactionRepository, transactionLogsRepository)
	transactionService := service.NewTransactionService(transactionHandler)
	grpcServer := server.NewGRPCServer(confServer, accountService, transactionService, logger)
	httpServer := server.NewHTTPServer(confServer, accountService, transactionService, logger)
	app := newApp(logger, grpcServer, httpServer)
	return app, func() {
		cleanup2()
		cleanup()
	}, nil
}
