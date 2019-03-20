package internal

import (
	"context"
	"github.com/InVisionApp/go-health"
	"github.com/InVisionApp/go-health/handlers"
	"github.com/ProtocolONE/geoip-service/pkg"
	"github.com/ProtocolONE/geoip-service/pkg/proto"
	metrics "github.com/ProtocolONE/go-micro-plugins/wrapper/monitoring/prometheus"
	"github.com/ProtocolONE/rabbitmq/pkg"
	"github.com/micro/go-micro"
	k8s "github.com/micro/kubernetes/go/micro"
	"github.com/paysuper/paysuper-billing-server/internal/config"
	"github.com/paysuper/paysuper-billing-server/internal/database"
	"github.com/paysuper/paysuper-billing-server/internal/service"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/grpc"
	"github.com/paysuper/paysuper-recurring-repository/pkg/constant"
	"github.com/paysuper/paysuper-recurring-repository/pkg/proto/repository"
	taxPkg "github.com/paysuper/paysuper-tax-service/pkg"
	"github.com/paysuper/paysuper-tax-service/proto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"log"
	"net/http"
	"time"
)

type Application struct {
	cfg        *config.Config
	database   *database.Source
	service    micro.Service
	httpServer *http.Server
	router     *http.ServeMux

	cacheExit chan bool
	logger    *zap.Logger
}

type appHealthCheck struct{}

func NewApplication() *Application {
	return &Application{cacheExit: make(chan bool, 1)}
}

func (app *Application) Init() {
	app.initLogger()

	cfg, err := config.NewConfig()

	if err != nil {
		app.logger.Fatal("[PAYONE_BILLING] Config load failed", zap.Error(err))
	}

	app.cfg = cfg
	app.initDatabase()

	broker, err := rabbitmq.NewBroker(app.cfg.BrokerAddress)

	if err != nil {
		app.logger.Fatal("Creating RabbitMQ publisher failed", zap.Error(err), zap.String("broker_address", app.cfg.BrokerAddress))
	}

	options := []micro.Option{
		micro.Name(pkg.ServiceName),
		micro.Version(pkg.ServiceVersion),
		micro.WrapHandler(metrics.NewHandlerWrapper()),
		micro.AfterStop(func() error {
			app.logger.Info("Micro service stopped")
			return nil
		}),
	}

	if app.cfg.MicroRegistry == constant.RegistryKubernetes {
		app.service = k8s.NewService(options...)
		app.logger.Info("[PAYSUPER_REPOSITORY] Initialize k8s service")
	} else {
		app.service = micro.NewService(options...)
		app.logger.Info("[PAYSUPER_REPOSITORY] Initialize micro service")
	}

	app.service.Init()

	geoService := proto.NewGeoIpService(geoip.ServiceName, app.service.Client())
	repService := repository.NewRepositoryService(constant.PayOneRepositoryServiceName, app.service.Client())
	taxService := tax_service.NewTaxService(taxPkg.ServiceName, app.service.Client())

	svc := service.NewBillingService(
		app.database,
		app.cfg,
		app.cacheExit,
		geoService,
		repService,
		taxService,
		broker,
	)

	if err := svc.Init(); err != nil {
		app.logger.Fatal("[PAYONE_BILLING] Create service instance failed", zap.Error(err))
	}

	err = grpc.RegisterBillingServiceHandler(app.service.Server(), svc)

	if err != nil {
		app.logger.Fatal("[PAYONE_BILLING] Service init failed", zap.Error(err))
	}

	app.router = http.NewServeMux()
	app.initHealth()
	app.initMetrics()
}

func (app *Application) initLogger() {
	var err error

	app.logger, err = zap.NewProduction()

	if err != nil {
		log.Fatalf("[PAYONE_BILLING] Application logger initialization failed with error: %s\n", err)
	}
	zap.ReplaceGlobals(app.logger)
}

func (app *Application) initDatabase() {
	settings := database.Connection{
		Host:     app.cfg.MongoHost,
		Database: app.cfg.MongoDatabase,
		User:     app.cfg.MongoUser,
		Password: app.cfg.MongoPassword,
	}

	db, err := database.NewDatabase(settings)

	if err != nil {
		app.logger.Fatal(
			"[PAYONE_BILLING] Database connection failed",
			zap.Error(err),
			zap.String("connection_string", settings.String()),
		)
	}

	app.database = db
}

func (app *Application) initHealth() {
	h := health.New()
	err := h.AddChecks([]*health.Config{
		{
			Name:     "health-check",
			Checker:  &appHealthCheck{},
			Interval: time.Duration(1) * time.Second,
			Fatal:    true,
		},
	})

	if err != nil {
		app.logger.Fatal("[PAYONE_BILLING] Health check register failed", zap.Error(err))
	}

	if err = h.Start(); err != nil {
		app.logger.Fatal("[PAYONE_BILLING] Health check start failed", zap.Error(err))
	}

	app.logger.Info("[PAYONE_BILLING] Health check listener started", zap.String("port", app.cfg.MetricsPort))

	app.router.HandleFunc("/health", handlers.NewJSONHandlerFunc(h, nil))
}

func (app *Application) initMetrics() {
	app.router.Handle("/metrics", promhttp.Handler())
}

func (app *Application) Run() {
	app.httpServer = &http.Server{
		Addr:    ":" + app.cfg.MetricsPort,
		Handler: app.router,
	}

	go func() {
		if err := app.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			app.logger.Fatal("[PAYONE_BILLING] Http server starting failed", zap.Error(err))
		}
	}()

	if err := app.service.Run(); err != nil {
		app.logger.Fatal("[PAYONE_BILLING] Micro service starting failed", zap.Error(err))
	}
}

func (app *Application) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := app.httpServer.Shutdown(ctx); err != nil {
		app.logger.Fatal("Http server shutdown failed", zap.Error(err))
	}
	app.logger.Info("Http server stopped")

	app.cacheExit <- true
	app.logger.Info("Cache rebuilding stopped")

	app.database.Close()
	app.logger.Info("Database connection closed")

	func() {
		if err := app.logger.Sync(); err != nil {
			app.logger.Fatal("Logger sync failed", zap.Error(err))
		} else {
			app.logger.Info("Logger synced")
		}

	}()
}

func (c *appHealthCheck) Status() (interface{}, error) {
	return "ok", nil
}
