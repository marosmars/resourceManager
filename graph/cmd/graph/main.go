// Copyright (c) 2004-present Facebook All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	stdlog "log"
	"net"
	"os"
	"syscall"

	"github.com/marosmars/resourceManager/ctxgroup"
	"github.com/marosmars/resourceManager/ctxutil"
	"github.com/marosmars/resourceManager/log"
	"github.com/marosmars/resourceManager/mysql"
	"github.com/marosmars/resourceManager/server"
	"github.com/marosmars/resourceManager/telemetry"
	"github.com/marosmars/resourceManager/viewer"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"gopkg.in/alecthomas/kingpin.v2"

	_ "github.com/marosmars/resourceManager/ent/runtime"
)

type cliFlags struct {
	HTTPAddress     *net.TCPAddr
	GRPCAddress     *net.TCPAddr
	MySQLConfig     mysql.Config
	LogConfig       log.Config
	TelemetryConfig telemetry.Config
	TenancyConfig   viewer.Config
}

func main() {
	var cf cliFlags
	kingpin.HelpFlag.Short('h')
	kingpin.Flag(
		"web.listen-address",
		"Web address to listen on",
	).
		Default(":http").
		TCPVar(&cf.HTTPAddress)
	kingpin.Flag(
		"grpc.listen-address",
		"GRPC address to listen on",
	).
		Default(":https").
		TCPVar(&cf.GRPCAddress)
	kingpin.Flag(
		"mysql.dsn",
		"mysql connection string",
	).
		Envar("MYSQL_DSN").
		Required().
		SetValue(&cf.MySQLConfig)

	log.AddFlagsVar(kingpin.CommandLine, &cf.LogConfig)
	telemetry.AddFlagsVar(kingpin.CommandLine, &cf.TelemetryConfig)
	viewer.AddFlagsVar(kingpin.CommandLine, &cf.TenancyConfig)
	kingpin.Parse()

	ctx := ctxutil.WithSignal(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	app, cleanup, err := newApplication(ctx, &cf)
	if err != nil {
		stdlog.Fatal(err)
	}
	defer cleanup()

	app.Info("starting application",
		zap.Stringer("http", cf.HTTPAddress),
		zap.Stringer("grpc", cf.GRPCAddress),
	)
	err = app.run(ctx)
	app.Info("terminating application", zap.Error(err))
}

type application struct {
	*zap.Logger
	http struct {
		*server.Server
		addr string
	}
	grpc struct {
		*grpc.Server
		addr string
	}
}

func (app *application) run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	g := ctxgroup.WithContext(ctx)
	g.Go(func(context.Context) error {
		err := app.http.ListenAndServe(app.http.addr)
		app.Debug("http server terminated", zap.Error(err))
		return err
	})
	g.Go(func(context.Context) error {
		lis, err := net.Listen("tcp", app.grpc.addr)
		if err != nil {
			return fmt.Errorf("creating grpc listener: %w", err)
		}
		err = app.grpc.Serve(lis)
		app.Debug("grpc server terminated", zap.Error(err))
		return err
	})
	g.Go(func(ctx context.Context) error {
		defer cancel()
		<-ctx.Done()
		return nil
	})
	<-ctx.Done()

	app.Warn("start application termination",
		zap.NamedError("reason", ctx.Err()),
	)
	defer app.Debug("end application termination")

	g.Go(func(context.Context) error {
		app.Debug("start grpc server termination")
		app.grpc.GracefulStop()
		app.Debug("end grpc server termination")
		return nil
	})
	g.Go(func(context.Context) error {
		app.Debug("start http server termination")
		err := app.http.Shutdown(context.Background())
		app.Debug("end http server termination", zap.Error(err))
		return err
	})
	return g.Wait()
}
